require 'spec_helper'

module BitsService
  describe Routes::AppStash do
    let(:blobstore) { double(Blobstore::Client) }
    let(:headers) { Hash.new }
    before do
      allow_any_instance_of(Routes::AppStash).to receive(:app_stash_blobstore).and_return(blobstore)
    end

    describe 'POST /app_stash/entries' do
      let(:tmp_dir) { '/path/to/tmp/dir' }
      let(:zip_filepath) { '/path/to/zip/file' }
      let(:request_body) { { application: 'something' } }
      let(:receipt_contents) { [{ 'fn' => 'foo', 'sha1' => '1234', 'mode' => '766' }] }

      before do
        allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(zip_filepath)
        allow(Dir).to receive(:mktmpdir).and_return(tmp_dir)
        allow(AppPackager).to receive(:unzip)
        allow(blobstore).to receive(:cp_r_to_blobstore)
        allow(FileUtils).to receive(:rm_r)
        allow_any_instance_of(Receipt).to receive(:contents).and_return(receipt_contents)
      end

      it 'returns HTTP status 201' do
        post '/app_stash/entries', request_body, headers
        expect(last_response.status).to eq(201)
      end

      it 'returns a list of SHAs' do
        post '/app_stash/entries', request_body, headers
        response_body = last_response.body
        expect(response_body).to_not be_empty

        json = JSON.parse(last_response.body)
        expect(json).to_not be_empty
        expect(json).to eq(receipt_contents)
      end

      it 'unzips the uploaded zip file' do
        expect(AppPackager).to receive(:unzip).with(zip_filepath, tmp_dir)
        post '/app_stash/entries', request_body, headers
      end

      it 'uploads the unzipped app files to the blobstore' do
        expect(blobstore).to receive(:cp_r_to_blobstore).with(tmp_dir)
        post '/app_stash/entries', request_body, headers
      end

      it 'removes the temporary folder' do
        expect(FileUtils).to receive(:rm_r).with(tmp_dir)
        post '/app_stash/entries', request_body, headers
      end

      context 'when the upload_filepath is nil' do
        before(:each) do
          allow_any_instance_of(Helpers::Upload::Params).to receive(:upload_filepath).and_return(nil)
        end

        it 'returns HTTP status 400' do
          post '/app_stash/entries', request_body, headers
          expect(last_response.status).to eq(400)
        end

        it 'returns a corresponding error' do
          post '/app_stash/entries', request_body, headers
          json = JSON.parse(last_response.body)
          expect(json['code']).to eq(160001)
          expect(json['description']).to eq('The app upload is invalid: missing key `application`')
        end

        it 'does not create a temporary dir' do
          expect(Dir).to_not receive(:mktmpdir)
          post '/app_stash/entries', request_body, headers
        end
      end

      context 'when the AppPackager raises an error' do
        before do
          allow(AppPackager).to receive(:unzip).and_raise(Errors::ApiError.new_from_details('AppBitsUploadInvalid', 'failed here'))
        end

        it 'returns HTTP status 400' do
          post '/app_stash/entries', request_body, headers
          expect(last_response.status).to eq(400)
        end

        it 'returns a corresponding error' do
          post '/app_stash/entries', request_body, headers
          json = JSON.parse(last_response.body)
          expect(json['code']).to eq(160001)
          expect(json['description']).to eq('The app upload is invalid: failed here')
        end

        it 'removes the temporary folder' do
          expect(FileUtils).to receive(:rm_r).with(tmp_dir)
          post '/app_stash/entries', request_body, headers
        end
      end

      context 'when copying the files to the blobstore fails' do
        before do
          allow(blobstore).to receive(:cp_r_to_blobstore).and_raise(StandardError.new('failed here'))
        end

        it 'return HTTP status 500' do
          post '/app_stash/entries', request_body, headers
          expect(last_response.status).to eq(500)
        end

        it 'removes the temporary folder' do
          expect(FileUtils).to receive(:rm_r).with(tmp_dir)
          post '/app_stash/entries', request_body, headers
        end
      end

      context 'when Receipt raises an error' do
        before do
          allow_any_instance_of(Receipt).to receive(:contents).and_raise(StandardError.new('failed here'))
        end

        it 'return HTTP status 500' do
          post '/app_stash/entries', request_body, headers
          expect(last_response.status).to eq(500)
        end
      end

      context 'when the blobstore disk is full' do
        before do
          allow(blobstore).to receive(:cp_r_to_blobstore).and_raise(Errno::ENOSPC)
        end

        it 'return HTTP status 507' do
          post '/app_stash/entries', request_body, headers
          expect(last_response.status).to eq(507)
          payload = JSON(last_response.body)
          expect(payload['code']).to eq 500000
          expect(payload['description']).to eq 'No space left on device'
        end

        it 'removes the temporary folder' do
          expect(FileUtils).to receive(:rm_r).with(tmp_dir)
          post '/app_stash/entries', request_body, headers
        end
      end
    end

    describe 'POST /app_stash/matches' do
      let(:request_body) { [].to_json }
      let(:headers) { { content_type: :json } }
      let(:resources) { [{ 'sha1' => 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' }, { 'sha1' => 'zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz' }] }

      before(:each) do
        allow(JSON).to receive(:parse).and_return(resources)
        allow(blobstore).to receive(:exists?).with('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa').and_return(true)
        allow(blobstore).to receive(:exists?).with('zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz').and_return(false)
      end

      it 'returns HTTP status 200' do
        post '/app_stash/matches', request_body, headers
        expect(last_response.status).to eq(200)
      end

      it 'returns a valid response body' do
        post '/app_stash/matches', request_body, headers

        allow(JSON).to receive(:parse).and_call_original
        payload = JSON.parse(last_response.body)
        expect(payload).to eq([{ 'sha1' => 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' }])
      end

      it 'makes sure the request payload is an Array' do
        expect(resources).to receive(:is_a?).with(Array)
        post '/app_stash/matches', request_body, headers
      end

      it 'validates all SHAs' do
        expect_any_instance_of(Routes::AppStash).to receive(:valid_sha?).exactly(resources.length).times
        post '/app_stash/matches', request_body, headers
      end

      it 'checks for all resources if they exists in the blobstore' do
        expect(blobstore).to receive(:exists?).with('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
        expect(blobstore).to receive(:exists?).with('zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz')

        post '/app_stash/matches', request_body, headers
      end

      context 'when the payload is not an array' do
        before do
          allow(resources).to receive(:is_a?).with(Array).and_return(false)
        end

        it 'returns HTTP status 422' do
          post '/app_stash/matches', request_body, headers
          expect(last_response.status).to eq(422)
        end

        it 'returns a corresponding error' do
          post '/app_stash/matches', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/The request is semantically invalid:/)
        end
      end

      context 'when the parsing the request body fails' do
        before(:each) do
          allow(JSON).to receive(:parse).and_raise(JSON::ParserError.new('failed here'))
        end

        it 'returns HTTP status 400' do
          post '/app_stash/matches', request_body, headers
          expect(last_response.status).to eq(400)
        end

        it 'returns a corresponding error' do
          post '/app_stash/matches', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/Request invalid due to parse error:/)
        end
      end

      context 'when the blobstore call fails' do
        before(:each) do
          allow(blobstore).to receive(:exists?).and_raise(StandardError.new('failed here'))
        end

        it 'return HTTP status 500' do
          post '/app_stash/matches', request_body, headers
          expect(last_response.status).to eq(500)
        end
      end
    end

    describe 'POST /app_stash/bundles' do
      let(:request_body) { [].to_json }
      let(:headers) { { content_type: :json } }
      let(:resources) { [{ 'fn' => 'app.rb', 'sha1' => 'existing' }, { 'fn' => 'lib.rb', 'sha1' => 'existing', 'mode' => '666' }] }
      let(:output_file) { double(:output_file) }

      let(:destination_dir) { 'mock_destination_dir' }
      let(:zip_dir) { 'mock_zip_dir' }
      let(:zip_path) { File.join(zip_dir, 'bundle.zip') }

      before(:each) do
        allow(JSON).to receive(:parse).and_return(resources)
        allow(blobstore).to receive(:exists?).with('existing').and_return(true)
        allow(blobstore).to receive(:exists?).with('non-existing').and_return(false)
        allow(blobstore).to receive(:download_from_blobstore)
        allow(AppPackager).to receive(:zip)
        allow(Dir).to receive(:mktmpdir).and_return(destination_dir, zip_dir)
        allow(File).to receive(:open).with(zip_path).and_return(output_file)
        allow(FileUtils).to receive(:rm_rf)
        allow(output_file).to receive(:read).and_return('')
      end

      it 'checks if each resource exists in the blobstore' do
        expect(blobstore).to receive(:exists?).with('existing').twice
        post '/app_stash/bundles', request_body, headers
      end

      it 'downloads each resource from the blobstore' do
        expect(blobstore).to receive(:download_from_blobstore).with('existing', anything, mode: Routes::AppStash::DEFAULT_FILE_MODE).once
        expect(blobstore).to receive(:download_from_blobstore).with('existing', anything, mode: '666'.to_i(8)).once
        post '/app_stash/bundles', request_body, headers
      end

      it 'zips the download directory' do
        expect(AppPackager).to receive(:zip).with(destination_dir, zip_path)
        post '/app_stash/bundles', request_body, headers
      end

      it 'returns HTTP status 200' do
        post '/app_stash/bundles', request_body, headers
        expect(last_response.status).to eq(200)
      end

      it 'deletes the download temporary folder' do
        expect(FileUtils).to receive(:rm_rf).with(destination_dir)
        post '/app_stash/bundles', request_body, headers
      end

      it 'deletes created zip file' do
        expect(FileUtils).to receive(:rm_rf).with(zip_path)
        post '/app_stash/bundles', request_body, headers
      end

      it 'returns the file' do
        allow(output_file).to receive(:read).and_return('OK')
        post '/app_stash/bundles', request_body, headers
        expect(last_response.body).to eq('OK')
      end

      context 'when the payload is not an array' do
        before do
          allow(resources).to receive(:is_a?).with(Array).and_return(false)
        end

        it 'returns HTTP status 422' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(422)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/The request is semantically invalid:/)
        end
      end

      context 'when the request is an empty list' do
        let(:resources) { [] }

        it 'returns HTTP status 422' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(422)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/The request is semantically invalid:/)
        end
      end

      context 'when a request entry is missing the sha1 key' do
        let(:resources) { [{ 'fn' => 'existing' }, { 'fn' => 'lib.rb', 'sha1' => 'existing' }] }

        it 'returns HTTP status 422' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(422)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/The request is semantically invalid:/)
        end
      end

      context 'when a request entry is missing the fn key' do
        let(:resources) { [{ 'sha1' => 'existing' }, { 'fn' => 'lib.rb', 'sha1' => 'existing' }] }

        it 'returns HTTP status 422' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(422)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/The request is semantically invalid:/)
        end
      end

      context 'when the blobstore cannot find a requested sha1' do
        let(:resources) { [{ 'fn' => 'app.rb', 'sha1' => 'existing' }, { 'fn' => 'lib.rb', 'sha1' => 'non-existing' }] }

        it 'returns HTTP status 404' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(404)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/non-existing not found/)
        end
      end

      context 'when the parsing the request body fails' do
        before(:each) do
          allow(JSON).to receive(:parse).and_raise(JSON::ParserError.new('failed here'))
        end

        it 'returns HTTP status 400' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(400)
        end

        it 'returns a corresponding error' do
          post '/app_stash/bundles', request_body, headers

          allow(JSON).to receive(:parse).and_call_original
          description = JSON.parse(last_response.body)['description']

          expect(description).to match(/Request invalid due to parse error:/)
        end
      end

      context 'when the blobstore call fails' do
        before(:each) do
          allow(blobstore).to receive(:download_from_blobstore).and_raise(StandardError.new('failed here'))
        end

        it 'return HTTP status 500' do
          post '/app_stash/bundles', request_body, headers
          expect(last_response.status).to eq(500)
        end
      end
    end
  end
end
