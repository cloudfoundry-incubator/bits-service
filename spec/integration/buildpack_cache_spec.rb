require 'spec_helper'

describe 'buildpack_cache resource', type: :integration do
  before(:all) do
    @root_dir = Dir.mktmpdir

    config = {
      droplets: {
        directory_key: 'directory-key',
        fog_connection: {
          provider: 'local',
          local_root: @root_dir
        }
      },
      nginx: {
        use_nginx: false
      }
    }

    start_server(config)
  end

  after(:all) do
    stop_server
    FileUtils.rm_rf(@root_dir)
  end

  after(:each) do
    FileUtils.rm_rf(File.dirname(zip_filepath))
    FileUtils.rm_rf(@root_dir)
    @root_dir = Dir.mktmpdir
  end

  let(:zip_filepath) { File.join(Dir.mktmpdir, 'file.zip') }

  let(:zip_file) do
    TestZip.create(zip_filepath, 1, 1024)
    File.new(zip_filepath)
  end

  let(:upload_body) { { buildpack_cache: zip_file } }
  let(:resource_path) { "#{collection_path}/#{key}" }
  let(:collection_path) { '/buildpack_cache/entries' }
  let(:guid) { SecureRandom.uuid }
  let(:key) { "#{guid}/some-stack-name" }

  def blobstore_path(key)
    blob_path(@root_dir, File.join('directory-key', 'buildpack_cache'), key)
  end

  describe 'PUT /buildpack_cache/entries' do
    it 'returns HTTP status 201' do
      response = make_put_request(resource_path, upload_body)
      expect(response.code).to eq 201
    end

    it 'correctly stores the file in the blob store' do
      make_put_request(resource_path, upload_body)

      expected_path = blobstore_path(key)
      expect(File).to exist(expected_path)
    end

    context 'when an empty request body is being sent' do
      let(:upload_body) { {} }

      it 'returns HTTP status 400' do
        response = make_put_request(resource_path, upload_body)
        expect(response.code).to eq 400
      end

      it 'returns the expected error description' do
        response = make_put_request(resource_path, upload_body)
        description = JSON.parse(response.body)['description']
        expect(description).to eq 'The buildpack_cache upload is invalid: a file must be provided'
      end
    end
  end

  describe 'GET /buildpack_cache/entries/:app_guid/:stack_name' do
    context 'when the buildpack cache exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 200' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 200
      end

      it 'returns the correct buildpack cache' do
        response = make_get_request(resource_path)
        expect(response.body).to eq(File.open(zip_filepath, 'rb').read)
      end
    end

    context 'when the buildpack cache does not exist' do
      it 'returns HTTP status code 404' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 404
      end
    end

    context 'when the stack name is missing' do
      it 'returns HTTP status code 404' do
        response = make_get_request('/buildpack_cache/entries/not-here')
        expect(response.code).to eq 404
      end
    end
  end

  describe 'DELETE /buildpack_cache/entries/:app_guid/:stack_name' do
    context 'when the buildpack cache exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 204' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 204
      end

      it 'removes the stored file' do
        expected_path = blobstore_path(key)
        expect(File).to exist(expected_path)
        make_delete_request(resource_path)
        expect(File).to_not exist(expected_path)
      end
    end

    context 'when the buildpack cache does not exist' do
      let(:resource_path) { '/buildpack_cache/entries/not-existing/windows' }

      it 'returns HTTP status code 404' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 404
      end
    end
  end

  describe 'DELETE /buildpack_cache/entries/:app_guid' do
    let(:resource_path_short) { "#{collection_path}/#{guid}" }

    context 'when the buildpack cache exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 204' do
        response = make_delete_request(resource_path_short)
        expect(response.code).to eq 204
      end

      it 'removes the stored file' do
        expected_path = blobstore_path(key)
        expect(File).to exist(expected_path)
        make_delete_request(resource_path_short)
        expect(File).to_not exist(expected_path)
      end
    end

    context 'when the buildpack cache does not exist' do
      let(:resource_path) { '/buildpack_cache/entries/not-existing' }

      it 'is a no-op and returns HTTP status code 204' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 204
      end
    end
  end

  describe 'DELETE /buildpack_cache/entries' do
    let(:key1) { "#{SecureRandom.uuid}/some-stack-name" }
    let(:key2) { "#{SecureRandom.uuid}/some-stack-name" }

    def create_file_for_upload
      filepath = File.join(Dir.mktmpdir, 'file.zip')
      TestZip.create(filepath, 1, 1024)
      File.new(filepath)
    end

    before do
      [key1, key2].each do |key|
        make_put_request("/buildpack_cache/entries/#{key}", { buildpack_cache: create_file_for_upload })
      end
    end

    it 'returns HTTP status 204' do
      response = make_delete_request(collection_path)
      expect(response.code).to eq 204
    end

    it 'removes all the stored files' do
      [key1, key2].each { |key| expect(File).to exist(blobstore_path(key)) }
      make_delete_request(collection_path)
      [key1, key2].each { |key| expect(File).to_not exist(blobstore_path(key)) }
    end
  end
end
