require 'spec_helper'
require 'securerandom'

module BitsService
  module Routes
    describe Buildpacks do
      let(:headers) { Hash.new }

      let(:zip_filename) { 'file.zip' }

      let(:zip_filepath) do
        path = File.join(Dir.mktmpdir, zip_filename)
        TestZip.create(path, 1, 1024)
        path
      end

      let(:zip_file) do
        Rack::Test::UploadedFile.new(File.new(zip_filepath))
      end

      let(:non_zip_file) do
        Rack::Test::UploadedFile.new(Tempfile.new('foo'))
      end

      let(:zip_file_sha) { Digester.new.digest_path(zip_file) }

      let(:guid) { SecureRandom.uuid }

      let(:upload_body) { { buildpack: zip_file, buildpack_name: zip_filename } }

      let(:use_nginx) { false }

      let(:config) do
        {
          buildpacks: {
            fog_connection: {
              provider: 'AWS',
              aws_access_key_id: 'fake_aws_key_id',
              aws_secret_access_key: 'fake_secret_access_key'
            }
          },
          nginx: {
            use_nginx: use_nginx
          }
        }
      end

      around(:each) do |example|
        config_filepath = create_config_file(config)
        BitsService::Environment.load_configuration(config_filepath)
        Fog.mock!

        example.run

        Fog.unmock!
        FileUtils.rm_f(config_filepath)
      end

      after(:each) do
        FileUtils.rm_rf(File.dirname(zip_filepath))
        FileUtils.rm_f(non_zip_file.tempfile.path)
      end

      describe 'PUT /buildpacks/:guid' do
        before do
          allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
          allow_any_instance_of(Helpers::Upload::Params).to receive(:original_filename).and_return(zip_filename)
        end

        it 'returns HTTP status 201' do
          put "/buildpacks/#{guid}", upload_body, headers
          expect(last_response.status).to eq(201)
        end

        it 'stores the uploaded file in the buildpack blobstore using the correct key' do
          blobstore = double(BitsService::Blobstore::Client)
          expect_any_instance_of(Routes::Buildpacks).to receive(:buildpack_blobstore).and_return(blobstore)
          expect(blobstore).to receive(:cp_to_blobstore).with(zip_filepath, guid)

          put "/buildpacks/#{guid}", upload_body, headers
        end

        it 'instantiates the upload params decorator with the right arguments' do
          expect(Helpers::Upload::Params).to receive(:new).with(hash_including(
                                                                  'buildpack' => anything,
                                                                  'buildpack_name' => zip_filename
          ), use_nginx: false).once

          put "/buildpacks/#{guid}", upload_body, headers
        end

        it 'gets the uploaded filepath from the upload params decorator' do
          decorator = double(Helpers::Upload::Params)
          allow(Helpers::Upload::Params).to receive(:new).and_return(decorator)
          expect(decorator).to receive(:upload_filepath).with('buildpack').once
          put "/buildpacks/#{guid}", upload_body, headers
        end

        it 'does not leave the temporary instance of the uploaded file around' do
          allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
          put "/buildpacks/#{guid}", upload_body, headers
          expect(File.exist?(zip_filepath)).to be_falsy
        end

        context 'when no file is being uploaded' do
          before(:each) do
            allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(nil)
          end

          it 'returns a corresponding error' do
            expect_any_instance_of(Routes::Buildpacks).to_not receive(:buildpack_blobstore)

            put "/buildpacks/#{guid}", upload_body, headers

            expect(last_response.status).to eq(400)
            json = JSON.parse(last_response.body)
            expect(json['code']).to eq(290_002)
            expect(json['description']).to match(/a file must be provided/)
          end
        end

        context 'when the blobstore copy fails' do
          before(:each) do
            allow_any_instance_of(Blobstore::Client).to receive(:cp_to_blobstore).and_raise('some error')
          end

          it 'return HTTP status 500' do
            put "/buildpacks/#{guid}", upload_body, headers
            expect(last_response.status).to eq(500)
          end

          it 'does not leave the temporary instance of the uploaded file around' do
            allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
            allow_any_instance_of(Helpers::Upload::Params).to receive(:original_filename).and_return(zip_filename)
            put "/buildpacks/#{guid}", upload_body, headers
            expect(File.exist?(zip_filepath)).to be_falsy
          end
        end

        context 'when the blobstore helper fails' do
          before(:each) do
            allow_any_instance_of(Routes::Buildpacks).to receive(:buildpack_blobstore).and_raise('some error')
          end

          it 'return HTTP status 500' do
            put "/buildpacks/#{guid}", upload_body, headers
            expect(last_response.status).to eq(500)
          end

          it 'does not leave the temporary instance of the uploaded file around' do
            allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
            allow_any_instance_of(Helpers::Upload::Params).to receive(:original_filename).and_return(zip_filename)
            put "/buildpacks/#{guid}", upload_body, headers
            expect(File.exist?(zip_filepath)).to be_falsy
          end
        end
      end

      describe 'GET /buildpacks/:guid' do
        let(:download_url) { 'some-url' }

        let(:blob) do
          double(BitsService::Blobstore::Blob, public_download_url: download_url)
        end

        let(:blobstore) do
          double(BitsService::Blobstore::Client).tap do |blobstore|
            allow(blobstore).to receive(:blob).with(guid).and_return(blob)
          end
        end

        before(:each) do
          allow_any_instance_of(Routes::Buildpacks).to receive(:buildpack_blobstore).and_return(blobstore)
        end

        it 'creates the buildpack blobstore using the blobstore factory' do
          expect_any_instance_of(Routes::Buildpacks).to receive(:buildpack_blobstore).at_least(:once)
          get "/buildpacks/#{guid}", headers
        end

        it 'finds the blob inside the blobstore using the correct guid' do
          expect(blobstore).to receive(:blob).with(guid)
          get "/buildpacks/#{guid}", headers
        end

        it 'checks whether the blobstore is local' do
          expect(blobstore).to receive(:local?).once
          get "/buildpacks/#{guid}", headers
        end

        context 'when the blobstore is local' do
          before(:each) do
            allow(blobstore).to receive(:local?).and_return(true)
          end

          context 'and we are using nginx' do
            let(:use_nginx) { true }

            let(:blob) do
              double(BitsService::Blobstore::Blob, internal_download_url: download_url)
            end

            it 'returns HTTP status code 200' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.status).to eq(200)
            end

            it 'sets the X-Accel-Redirect response header' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.headers).to include('X-Accel-Redirect' => download_url)
            end

            it 'gets the download_url from the blob' do
              expect(blob).to receive(:internal_download_url).once
              get "/buildpacks/#{guid}", headers
            end
          end

          context 'and we are not using nginx' do
            let(:use_nginx) { false }

            before(:each) do
              allow(blob).to receive(:local_path).and_return(zip_filepath)
            end

            it 'returns HTTP status code 200' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.status).to eq(200)
            end

            it 'sets the right Content-Type header' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.headers).to include('Content-Type' => 'application/zip')
            end

            it 'sets the right Content-Length header' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.headers).to include('Content-Length' => File.size(zip_filepath).to_s)
            end

            it 'returns the file contents in the response body' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.body).to eq(File.open(zip_filepath, 'rb').read)
            end

            it 'does not set the X-Accel-Redirect response header' do
              get "/buildpacks/#{guid}", headers
              expect(last_response.headers).to_not include('X-Accel-Redirect')
            end

            it 'gets the local_path from the blob' do
              expect(blob).to receive(:local_path).once
              get "/buildpacks/#{guid}", headers
            end
          end
        end

        context 'when the blobstore is remote' do
          before(:each) do
            allow(blobstore).to receive(:local?).and_return(false)
          end

          it 'returns HTTP status code 302' do
            get "/buildpacks/#{guid}", headers
            expect(last_response.status).to eq(302)
          end

          it 'sets the location header to the correct value' do
            get "/buildpacks/#{guid}", headers
            expect(last_response.headers).to include('Location' => download_url)
          end
        end

        context 'when the buildpack does not exist' do
          let(:blob) { nil }

          it 'returns a corresponding error' do
            get "/buildpacks/#{guid}", headers

            expect(last_response.status).to eq(404)
          end
        end
      end

      describe 'DELETE /buildpacks/:guid' do
        let(:blob) do
          double(BitsService::Blobstore::Blob)
        end

        let(:blobstore) do
          double(BitsService::Blobstore::Client, blob: blob)
        end

        before(:each) do
          allow_any_instance_of(Routes::Buildpacks).to receive(:buildpack_blobstore).and_return(blobstore)
          allow(blobstore).to receive(:delete_blob).and_return(true)
        end

        it 'returns HTTP status code 204' do
          delete "/buildpacks/#{guid}", headers
          expect(last_response.status).to eq(204)
        end

        it 'deletes the blob using the blobstore client' do
          expect(blobstore).to receive(:delete_blob).with(blob)
          delete "/buildpacks/#{guid}", headers
        end

        context 'when the buildpack does not exist' do
          let(:blob) { nil }

          it 'returns a corresponding error' do
            delete "/buildpacks/#{guid}", headers

            expect(last_response.status).to eq(404)
          end
        end
      end
    end
  end
end
