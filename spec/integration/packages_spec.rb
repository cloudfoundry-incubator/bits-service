require 'spec_helper'
require 'stub_server'
require 'open-uri'

describe 'packages resource', type: :integration do
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

  context 'without CC update config' do
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
        },
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

      context 'when the package does not exist' do
        let(:resource_path) { '/packages/not-existing' }

        it 'returns HTTP status code 404' do
          response = make_delete_request(resource_path)
          expect(response.code).to eq 404
        end
      end
    end
  end

  context 'with CC update config' do
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
        },
        cc_updates: {
          cc_url: 'https://localhost:9123',
          ca_cert: File.expand_path('../../certificates/ca.crt', __FILE__),
          client_cert: File.expand_path('../../certificates/bits-service.crt', __FILE__),
          client_key: File.expand_path('../../certificates/bits-service.key', __FILE__)
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

    let(:ssl) do
      {
        cert: File.read('spec/certificates/server.crt'),
        key: File.read('spec/certificates/server.key')
      }
    end

    let(:webrick_additional_config) do
      {
        SSLVerifyClient: OpenSSL::SSL::VERIFY_PEER | OpenSSL::SSL::VERIFY_FAIL_IF_NO_PEER_CERT,
        SSLCACertificateFile: 'spec/certificates/ca.crt',
      }
    end

    let(:status_code) { 204 }
    let(:replies) do
      { "/packages/#{guid}" => [status_code, {}, []] }
    end

    around(:example) do |example|
      StubServer.open('9123', replies, ssl: ssl, webrick: webrick_additional_config) do |server|
        server.wait
        example.run
      end
    end

    context 'setting status to `PROCESSING_UPLOAD` succeeds' do
      let(:status_code) { 204 }

      context 'with file upload' do
        it 'returns HTTP status 201' do
          response = make_put_request(resource_path, upload_body)
          expect(response.code).to eq 201
        end
      end
    end

    context 'setting status to `PROCESSING_UPLOAD` fails' do
      let(:status_code) { 422 }

      context 'with file upload' do
        it 'returns HTTP status 400' do
          response = make_put_request(resource_path, upload_body)
          expect(response.code).to eq 400
          expect(response.body).to include('Cannot update an existing package')
        end
      end
    end

    context 'CloudController fails' do
      let(:status_code) { 500 }

      context 'with file upload' do
        it 'returns HTTP status 500' do
          response = make_put_request(resource_path, upload_body)
          expect(response.code).to eq 500
        end
      end
    end

    context 'unknown package' do
      let(:status_code) { 404 }

      context 'with file upload' do
        it 'returns HTTP status 404' do
          response = make_put_request(resource_path, upload_body)
          expect(response.code).to eq 404
          expect { JSON.parse(response.body) }.to_not raise_error
          expect(JSON.parse(response.body)['code']).to eq 10010
        end
      end
    end
  end
end
