require 'spec_helper'

module BitsService
  module Routes
    describe Packages do
      let(:blobstore) { double(Blobstore::Client) }
      let(:headers) { Hash.new }
      let(:cc_updater) { double(CCUpdater) }
      let(:guid) { SecureRandom.uuid }

      before do
        allow_any_instance_of(Routes::Packages).to receive(:packages_blobstore).and_return(blobstore)
        allow_any_instance_of(Routes::Packages).to receive(:cc_updater).and_return(cc_updater)

        allow(cc_updater).to receive(:processing_upload)
        allow(cc_updater).to receive(:ready)
        allow(cc_updater).to receive(:failed)
      end

      describe 'create a new package' do
        context 'from an uploaded file' do
          let(:zip_filepath) { '/path/to/zip/file' }
          let(:request_body) { { application: 'something' } }
          let(:sha1) { double(Digest::SHA1) }
          let(:sha256) { double(Digest::SHA256) }

          subject(:response) do
            put "/packages/#{guid}", request_body, headers
          end

          before do
            allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
            allow(blobstore).to receive(:cp_to_blobstore)
            allow(FileUtils).to receive(:rm_r)
            allow(Digest::SHA1).to receive(:file).and_return(sha1)
            allow(Digest::SHA256).to receive(:file).and_return(sha256)
            allow(sha1).to receive(:hexdigest).and_return('dummy_sha1')
            allow(sha256).to receive(:hexdigest).and_return('dummy_sha256')
          end

          it 'returns HTTP status 201' do
            expect(response.status).to eq(201)
          end

          it 'updates the Cloud Controller' do
            expect(cc_updater).to receive(:processing_upload).with(guid)
            expect(cc_updater).to receive(:ready).with(guid, sha1: 'dummy_sha1', sha256: 'dummy_sha256')
            expect(cc_updater).to_not receive(:failed)
            expect(response).to be
          end

          context 'fails when updating the Cloud Controller with PROCESSING_UPLOAD' do
            it 'returns HTTP status 400' do
              expect(cc_updater).to receive(:processing_upload).with(guid).and_raise(CCUpdater::UpdateError)
              expect(response.status).to eq(400)
              expect(response.body).to include('Cannot update')
            end
          end

          context 'fails when updating the Cloud Controller with READY' do
            it 'returns HTTP status 400' do
              expect(cc_updater).to receive(:ready).with(guid, anything).and_raise(CCUpdater::UpdateError)
              expect(response.status).to eq(400)
              expect(response.body).to include('Cannot update')
            end
          end

          context 'when the upload_filepath is empty' do
            before do
              allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return('')
            end

            it 'returns HTTP status 400' do
              expect(response.status).to eq(400)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to receive(:processing_upload).with(guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to receive(:failed).with(guid, 'The package upload is invalid: a file must be provided')
              expect(response).to be
            end

            it 'returns a corresponding error' do
              json = JSON.parse(response.body)
              expect(json['description']).to eq('The package upload is invalid: a file must be provided')
            end

            it 'does not create a temporary dir' do
              expect(Dir).to_not receive(:mktmpdir)
              expect(response).to be
            end

            context 'fails when updating the Cloud Controller with FAILED' do
              it 'returns HTTP status 400 with the original error' do
                expect(cc_updater).to receive(:failed).with(guid, anything).and_raise(CCUpdater::UpdateError)
                expect(response.status).to eq(400)
                expect(response.body).to include('The package upload is invalid: a file must be provided')
              end
            end
          end

          context 'when copying the files to the blobstore fails' do
            before do
              allow(blobstore).to receive(:cp_to_blobstore).and_raise(StandardError.new('failed here'))
            end

            it 'return HTTP status 500' do
              expect(response.status).to eq(500)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to receive(:processing_upload).with(guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to receive(:failed).with(guid, 'failed here')
              expect(response).to be
            end

            it 'removes the temporary folder' do
              expect(FileUtils).to receive(:rm_f).with(zip_filepath)
              expect(response).to be
            end

            context 'fails when updating the Cloud Controller with FAILED' do
              it 'returns HTTP status 500 with the original error' do
                expect(cc_updater).to receive(:failed).with(guid, anything).and_raise(CCUpdater::UpdateError)
                expect(response.status).to eq(500)
                expect(response.body).to include('failed here')
              end
            end
          end

          context 'when the blobstore disk is full' do
            before do
              allow(blobstore).to receive(:cp_to_blobstore).and_raise(Errno::ENOSPC)
            end

            it 'return HTTP status 507' do
              expect(response.status).to eq(507)
              payload = JSON(response.body)
              expect(payload['code']).to eq 500000
              expect(payload['description']).to eq 'No space left on device'
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to receive(:processing_upload).with(guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to receive(:failed).with(guid, 'No space left on device')
              expect(response).to be
            end

            it 'removes the temporary folder' do
              expect(FileUtils).to receive(:rm_f).with(zip_filepath)
              expect(response).to be
            end

            context 'fails when updating the Cloud Controller with FAILED' do
              it 'returns HTTP status 507 with the original error' do
                expect(cc_updater).to receive(:failed).with(guid, anything).and_raise(CCUpdater::UpdateError)
                expect(response.status).to eq(507)
                expect(response.body).to include('No space left on device')
              end
            end
          end
        end

        context 'from another package' do
          let!(:guid) { SecureRandom.uuid }
          let!(:new_guid) { SecureRandom.uuid }

          let(:blob) { double(:blob) }
          let(:package_file) do
            Tempfile.new('package').tap do |file|
              file.write('content!')
              file.close
            end
          end

          subject(:response) do
            put "/packages/#{new_guid}", JSON.generate(source_guid: guid)
          end

          before do
            allow(blobstore).to receive(:blob).and_return(blob)
            allow(blobstore).to receive(:cp_file_between_keys)
          end

          it 'returns HTTP status 201' do
            expect(response.status).to eq(201)
          end

          it 'updates the Cloud Controller' do
            expect(cc_updater).to_not receive(:processing_upload) # .with(new_guid)
            # TODO: (ae, su) Should be :ready, wait till we figure out about the hashes
            expect(cc_updater).to_not receive(:ready) # .with(new_guid, { sha1: 'potato', sha256: 'potatoest' })
            expect(cc_updater).to_not receive(:failed)
            expect(response).to be
          end

          it 'copies the blob between keys' do
            expect(blobstore).to receive(:cp_file_between_keys).with(guid, new_guid)
            expect(response).to be
          end

          xcontext 'trying to update Cloud Controller fails' do
            it 'returns an error' do
              expect(blobstore).to receive(:cp_file_between_keys).with(guid, new_guid)
              expect(cc_updater).to_not receive(:ready) # .and_raise(CCUpdater::UpdateError)
              expect(response.status).to eq(400)
              expect(response.body).to include('Cannot update')
            end
          end

          context 'when the blob is missing' do
            before do
              allow(blobstore).to receive(:blob).and_return(nil)
            end

            it 'returns HTTP status 404' do
              expect(response.status).to eq(404)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(new_guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to_not receive(:failed) # .with(new_guid, guid)
              expect(response).to be
            end
          end

          context 'when copying the blob object fails' do
            before do
              allow(blobstore).to receive(:cp_file_between_keys).and_raise(StandardError.new('copying failed'))
            end

            it 'returns HTTP status 500' do
              expect(response.status).to eq(500)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(new_guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to_not receive(:failed) # .with(new_guid, 'copying failed')
              expect(response).to be
            end
          end

          context 'when local storage is full' do
            before do
              allow(blobstore).to receive(:cp_file_between_keys).and_raise(Errno::ENOSPC)
            end

            it 'returns HTTP status 507' do
              expect(response.status).to eq(507)
              payload = JSON(response.body)
              expect(payload['code']).to eq 500000
              expect(payload['description']).to eq 'No space left on device'
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(new_guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to_not receive(:failed) # .with(new_guid, 'No space left on device')
              expect(response).to be
            end
          end

          context 'when fetching the blob object fails' do
            before do
              allow(blobstore).to receive(:blob).and_raise(StandardError.new('fetching failed'))
            end

            it 'returns HTTP status 500' do
              expect(response.status).to eq(500)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(new_guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to_not receive(:failed) # .with(new_guid, 'fetching failed')
              expect(response).to be
            end
          end
        end

        context 'when both the blob and the source_guid are missing' do
          context 'with empty body' do
            subject(:response) { put "/packages/#{guid}", '' }

            it 'returns HTTP status 400' do
              expect(response.status).to eq(400)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to receive(:failed).with(guid, start_with('Cannot create package'))
              expect(response).to be
            end

            context 'fails when updating the Cloud Controller with FAILED' do
              it 'returns HTTP status 400 with the original error' do
                expect(cc_updater).to receive(:failed).with(guid, anything).and_raise(CCUpdater::UpdateError)
                expect(response.status).to eq(400)
                expect(response.body).to include('Cannot create package')
              end
            end
          end

          context 'with empty json' do
            subject(:response) { put "/packages/#{guid}", '{}' }

            it 'returns HTTP status 400' do
              expect(response.status).to eq(400)
            end

            it 'updates the Cloud Controller' do
              expect(cc_updater).to_not receive(:processing_upload) # .with(guid)
              expect(cc_updater).to_not receive(:ready)
              expect(cc_updater).to receive(:failed).with(guid, start_with('Cannot create package'))
              expect(response).to be
            end

            context 'fails when updating the Cloud Controller with FAILED' do
              it 'returns HTTP status 400 with the original error' do
                expect(cc_updater).to receive(:failed).with(guid, anything).and_raise(CCUpdater::UpdateError)
                expect(response.status).to eq(400)
                expect(response.body).to include('Cannot create package')
              end
            end
          end
        end
      end

      describe 'GET /packages' do
        let(:guid) { SecureRandom.uuid }
        let(:blob) { double(:blob) }
        let(:package_file) do
          Tempfile.new('package').tap do |file|
            file.write('content!')
            file.close
          end
        end
        subject(:response) { get "/packages/#{guid}" }

        before do
          allow(blobstore).to receive(:blob).and_return(blob)
          allow(blobstore).to receive(:local?).and_return(true)
          allow_any_instance_of(Packages).to receive(:use_nginx?).and_return(false)
          allow(blob).to receive(:local_path).and_return(package_file.path)
        end

        it 'returns HTTP status 200' do
          expect(response.status).to eq(200)
        end

        it 'returns the blob contents' do
          expect(response.body).to eq(File.read(package_file.path))
        end

        context 'when blobstore is not local' do
          let(:download_url) { 'http://blobstore.com/someblob' }

          before do
            allow(blobstore).to receive(:local?).and_return(false)
            allow(blob).to receive(:public_download_url).and_return(download_url)
          end

          it 'returns HTTP status 302' do
            expect(response.status).to eq(302)
          end

          it 'returns the blob url in the Location header' do
            expect(response.headers['Location']).to eq(download_url)
          end
        end

        context 'when the bits service is using NGINX' do
          let(:download_url) { 'http://blobstore.com/someblob' }

          before do
            allow_any_instance_of(Packages).to receive(:use_nginx?).and_return(true)
            allow(blob).to receive(:internal_download_url).and_return(download_url)
          end

          it 'returns HTTP status 200' do
            expect(response.status).to eq(200)
          end

          it 'returns the blob url in the X-Accel-Redirect header' do
            expect(response.headers['X-Accel-Redirect']).to eq(download_url)
          end
        end

        context 'when the blob is missing' do
          before do
            allow(blobstore).to receive(:blob).and_return(nil)
          end

          it 'returns HTTP status 404' do
            expect(response.status).to eq(404)
          end
        end

        context 'when fetching the blob object fails' do
          before do
            allow(blobstore).to receive(:blob).and_raise(StandardError)
          end

          it 'returns HTTP status 500' do
            expect(response.status).to eq(500)
          end
        end
      end

      describe 'DELETE /packages/:guid' do
        let(:guid) { SecureRandom.uuid }
        let(:blob) { double(:blob) }

        subject(:response) { delete "/packages/#{guid}", {} }

        before do
          allow(blobstore).to receive(:blob).and_return(blob)
          allow(blobstore).to receive(:delete_blob).and_return(blob)
        end

        it 'returns HTTP status 204' do
          expect(response.status).to eq(204)
        end

        it 'uses the correct key to fetch the blob' do
          expect(blobstore).to receive(:blob).with(guid)
          expect(response).to be
        end

        it 'asks for the package to be deleted' do
          expect(blobstore).to receive(:delete_blob).with(blob)
          expect(response).to be
        end

        context 'when the package does not exist' do
          before do
            allow(blobstore).to receive(:blob).and_return(nil)
          end

          it 'returns HTTP status 404' do
            expect(response.status).to eq(404)
          end
        end

        context 'when blobstore lookup fails' do
          before do
            allow(blobstore).to receive(:blob).and_raise
          end

          it 'returns HTTP status 500' do
            expect(response.status).to eq(500)
          end
        end

        context 'when deleting the blob fails' do
          before do
            allow(blobstore).to receive(:delete_blob).and_raise
          end

          it 'returns HTTP status 500' do
            expect(response.status).to eq(500)
          end
        end
      end
    end
  end
end
