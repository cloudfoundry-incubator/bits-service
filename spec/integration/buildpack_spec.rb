require 'spec_helper'

describe 'buildpacks resource', type: :integration do
  before(:all) do
    @root_dir = Dir.mktmpdir

    config = {
      buildpacks: {
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

  let(:guid) { SecureRandom.uuid }
  let(:upload_body) { { buildpack: zip_file, buildpack_name: 'original.zip' } }
  let(:zip_file_sha) { BitsService::Digester.new.digest_path(zip_file) }
  let(:resource_path) do
    "/buildpacks/#{guid}"
  end

  def blobstore_path(guid)
    blob_path(@root_dir, 'directory-key', guid)
  end

  describe 'POST /buildpack' do
    it 'returns HTTP status 201' do
      response = make_put_request(resource_path, upload_body)
      expect(response.code).to eq 201
    end

    it 'correctly stores the file in the blob store' do
      make_put_request(resource_path, upload_body)

      expected_path = blobstore_path(guid)
      expect(File).to exist(expected_path)
      expect(BitsService::Digester.new.digest_path(expected_path)).to eq zip_file_sha
    end

    context 'when an empty request body is being sent' do
      let(:upload_body) { { buildpack_name: 'original.zip' } }

      it 'returns HTTP status 400' do
        response = make_put_request(resource_path, upload_body)
        expect(response.code).to eq 400
      end

      it 'returns the expected error description' do
        response = make_put_request(resource_path, upload_body)
        description = JSON.parse(response.body)['description']
        expect(description).to eq 'The buildpack upload is invalid: a file must be provided'
      end
    end
  end

  describe 'GET /buildpacks/:guid' do
    context 'when the buildpack exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 200' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 200
      end

      it 'returns the correct bits' do
        response = make_get_request(resource_path)
        expect(response.body).to eq(File.open(zip_filepath, 'rb').read)
      end
    end

    context 'when the buildpack does not exist' do
      let(:resource_path) { '/buildpacks/not-existing' }

      it 'returns HTTP status code 404' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 404
      end
    end
  end

  describe 'DELETE /buildpacks/:guid' do
    context 'when the buildpack exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 204' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 204
      end

      it 'removes the stored file' do
        expected_path = blobstore_path(guid)
        expect(File).to exist(expected_path)
        make_delete_request(resource_path)
        expect(File).to_not exist(expected_path)
      end
    end

    context 'when the buildpack does not exist' do
      let(:resource_path) { '/buildpacks/not-existing' }

      it 'returns HTTP status code 404' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 404
      end
    end
  end
end
