require 'spec_helper'

describe 'packages resource', type: :integration do
  before(:all) do
    @root_dir = Dir.mktmpdir

    config = {
      packages: {
        directory_key: 'packages',
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
    FileUtils.rm_rf(@root_dir)
    @root_dir = Dir.mktmpdir
  end

  let(:package_contents) { 'package contents' }
  let(:package) do
    Tempfile.new('package.zip').tap do |file|
      file.write(package_contents)
      file.close
    end
  end
  let(:upload_body) { { package: File.new(package.path) } }
  let(:resource_path) { "/packages/#{guid}" }
  let(:guid) { SecureRandom.uuid }

  describe 'PUT /packages/:guid' do
    context 'with file upload' do
      it 'returns HTTP status 201' do
        response = make_put_request(resource_path, upload_body)
        expect(response.code).to eq 201
      end

      it 'stores the package in the package blobstore' do
        make_put_request(resource_path, upload_body)

        expected_path = blob_path(@root_dir, 'packages', guid)
        expect(File).to exist(expected_path)
        expect(File.read(expected_path)).to eq(package_contents)
      end

      context 'when the package attachment is missing' do
        it 'returns HTTP status 400' do
          response = make_put_request(resource_path, {})
          expect(response.code).to eq 400
        end

        it 'returns an error message' do
          response = make_put_request(resource_path, {})
          description = JSON.parse(response.body)['description']
          expect(description).to eq 'Cannot create package. The source must either be uploaded or the guid of a source app to be copied must be provided'
        end
      end
    end

    context 'with source_guid' do
      let(:source_guid) do
        SecureRandom.uuid.tap do |guid|
          make_put_request("/packages/#{guid}", upload_body)
        end
      end
      let(:update_body) { JSON.generate(source_guid: source_guid) }

      it 'returns HTTP status 201' do
        response = make_put_request(resource_path, update_body)
        expect(response.code).to eq 201
      end

      it 'stores the package in the package blobstore' do
        make_put_request(resource_path, update_body)

        expected_path = blob_path(@root_dir, 'packages', guid)
        expect(File).to exist(expected_path)
        expect(File.read(expected_path)).to eq(package_contents)
      end

      context 'when the package does not exist' do
        let(:source_guid) { 'invalid-guid' }

        it 'returns HTTP status 404' do
          response = make_put_request(resource_path, update_body)
          expect(response.code).to eq 404
        end
      end
    end
  end

  describe 'GET /packages/:guid' do
    context 'when the package exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 200' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 200
      end

      it 'returns the correct bits' do
        response = make_get_request(resource_path)
        expect(response.body).to eq(File.open(package.path, 'rb').read)
      end
    end

    context 'when the droplets does not exist' do
      let(:resource_path) { '/packages/not-existing' }

      it 'returns HTTP status code 404' do
        response = make_get_request(resource_path)
        expect(response.code).to eq 404
      end
    end
  end

  describe 'DELETE /packages/:guid' do
    context 'when the packages exists' do
      before do
        make_put_request(resource_path, upload_body)
      end

      it 'returns HTTP status code 204' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 204
      end

      it 'removes the stored file' do
        expected_path = blob_path(@root_dir, 'packages', guid)

        expect do
          make_delete_request(resource_path)
        end.to change {
          File.exist?(expected_path)
        }.from(true).to(false)
      end
    end

    context 'when the droplets does not exist' do
      let(:resource_path) { '/packages/not-existing' }

      it 'returns HTTP status code 404' do
        response = make_delete_request(resource_path)
        expect(response.code).to eq 404
      end
    end
  end
end
