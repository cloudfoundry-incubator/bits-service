require 'spec_helper'

describe 'app_stash endpoint', type: :integration do
  before(:all) do
    @root_dir = Dir.mktmpdir

    config = {
      app_stash: {
        directory_key: 'app_stash',
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

  let(:zip_filepath) { File.expand_path('../../fixtures/integration/app.zip', __FILE__) }
  let(:app_rb_sha) { '8b381f8864b572841a26266791c64ae97738a659' }
  let(:lib_rb_sha) { '594eb15515c89bbfb0874aa4fd4128bee0a1d0b5' }

  describe 'POST /app_stash/entries' do
    let(:request_body) { { application: File.new(zip_filepath) } }
    let(:app_rb_file_mode) { '777' }
    let(:lib_rb_file_mode) { '666' }

    it 'returns 201 status' do
      response = make_post_request('/app_stash/entries', request_body)
      expect(response.code).to eq(201)
    end

    it 'stores all the files from the zip based in their SHAs' do
      make_post_request('/app_stash/entries', request_body)

      app_rb = blob_path(@root_dir, File.join('app_stash', 'app_bits_cache'), app_rb_sha)
      expect(File.exist?(app_rb)).to eq(true)

      lib_rb = blob_path(@root_dir, File.join('app_stash', 'app_bits_cache'), lib_rb_sha)
      expect(File.exist?(lib_rb)).to eq(true)
    end

    it 'returns a list of SHAs for all files received' do
      response = make_post_request('/app_stash/entries', request_body)
      response_body = response.body
      expect(response_body).to_not be_empty
      json = JSON.parse(response_body)
      expect(json).to_not be_empty
      expect(json.size).to eq(2)

      expect(json).to include({ 'fn' => 'app/app.rb', 'sha1' => app_rb_sha, 'mode' => app_rb_file_mode })
      expect(json).to include({ 'fn' => 'app/lib.rb', 'sha1' => lib_rb_sha, 'mode' => lib_rb_file_mode })
    end

    context 'when the file is not a valid zip' do
      let(:zip_filepath) { File.expand_path('../../fixtures/integration/invalid.zip', __FILE__) }

      it 'returns 400 status' do
        response = make_post_request('/app_stash/entries', request_body)
        expect(response.code).to eq(400)
      end

      it 'returns an json describing the issue' do
        response = make_post_request('/app_stash/entries', request_body)
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The app upload is invalid: Invalid zip archive/)
      end
    end

    context 'when the file is missing' do
      let(:request_body) { Hash.new }

      it 'returns 400 status' do
        response = make_post_request('/app_stash/entries', request_body)
        expect(response.code).to eq(400)
      end

      it 'returns an json describing the issue' do
        response = make_post_request('/app_stash/entries', request_body)
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The app upload is invalid: missing key `application`/)
      end
    end
  end

  describe 'POST /app_stash/matches' do
    subject(:response) { make_post_request('/app_stash/matches', request_body, content_type: :json) }

    let(:existing_shas) { [app_rb_sha, lib_rb_sha] }
    let(:non_existing_shas) do
      ['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb']
    end

    before do
      zip_filepath = File.expand_path('../../fixtures/integration/app.zip', __FILE__)
      request_body = { application: File.new(zip_filepath) }
      make_post_request('/app_stash/entries', request_body)
    end

    def build_request_body(shas_array)
      shas_array.map { |sha| { sha1: sha, size: 123 } }.to_json
    end

    context 'when none of the resources exists in the stash' do
      let(:request_body) { build_request_body(non_existing_shas) }

      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end

      it 'reports no matches' do
        payload = JSON.parse(response.body)
        expect(payload).to be_a(Array)
        expect(payload).to be_empty
      end
    end

    context 'when some but not all of the resources exist in the stash' do
      let(:request_body) { build_request_body(non_existing_shas + existing_shas) }

      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end

      it 'reports only the existing resources as matches' do
        expect(response.body).to eq(build_request_body(existing_shas))
      end
    end

    context 'when all resources exist in the stash' do
      let(:request_body) { build_request_body(existing_shas) }

      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end

      it 'reports all resources as matches' do
        expect(response.body).to eq(request_body)
      end
    end

    context 'when the request includes invalid SHAs' do
      let(:request_body) { [{ sha1: '0', size: 0 }].to_json }

      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end
    end

    context 'when the body is empty' do
      let(:request_body) { nil }

      it 'returns HTTP status 400' do
        expect(response.code).to eq(400)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/Request invalid due to parse error:/)
      end
    end

    context 'when the body is invalid JSON' do
      let(:request_body) { 'some-invalid-json' }

      it 'returns HTTP status 400' do
        expect(response.code).to eq(400)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/Request invalid due to parse error:/)
      end
    end

    context 'when the body is not a JSON list' do
      let(:request_body) { { foo: 'bar' }.to_json }

      it 'returns HTTP status 422' do
        expect(response.code).to eq(422)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The request is semantically invalid:/)
      end
    end

    context 'when none of the entries has a digest' do
      let(:request_body) { [{ size: 10 }].to_json }

      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end

      it 'returns an empty JSON list' do
        payload = JSON.parse(response.body)
        expect(payload).to be_a(Array)
        expect(payload).to be_empty
      end
    end
  end

  describe 'POST /app_stash/bundles' do
    subject(:response) { make_post_request('/app_stash/bundles', request_body) }

    let(:existing_resources) do
      [
        { 'sha1' => app_rb_sha, 'fn' => 'app/app.rb' },
        { 'sha1' => lib_rb_sha, 'fn' => 'app/lib.rb', 'mode' => '777' },
      ]
    end

    let(:nonexisting_resources) do
      [
        { 'sha1' => 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'fn' => 'app/not-there' },
      ]
    end

    let(:request_body) { resources.to_json }

    let(:zip_filepath) { File.expand_path('../../fixtures/integration/app.zip', __FILE__) }

    let(:zip_file_sha1) { BitsService::Digester.new.digest_path(zip_filepath) }

    before(:each) do
      request_body = { application: File.new(zip_filepath) }
      make_post_request('/app_stash/entries', request_body)
    end

    def count_files_in_dir(dir)
      Dir[File.join(dir, '**', '*')].count { |file| File.file?(file) }
    end

    def write_response_body_to_file(response)
      Tempfile.new('zipfile').tap do |zip|
        zip.write(response.body)
        zip.close
      end
    end

    shared_examples 'bundles endpoint' do
      it 'returns HTTP status 200' do
        expect(response.code).to eq(200)
      end

      it 'returns an application package' do
        zip = write_response_body_to_file(response)
        unzip_path = Dir.mktmpdir('unzip')

        BitsService::AppPackager.unzip(zip.path, unzip_path)

        unzipped_files = Dir[File.join(unzip_path, '**', '*')].reject { |f| File.directory?(f) }.sort
        expected_files = resources.map { |spec| File.join(unzip_path, spec[:fn]) }.sort
        expect(unzipped_files).to eq(expected_files)

        resources.each do |spec|
          unzipped_file_path = File.join(unzip_path, spec[:fn])
          expect(BitsService::Digester.new.digest_path(unzipped_file_path)).to eq(spec[:sha1])

          if spec[:mode]
            expect(File.stat(unzipped_file_path).mode.to_s(8)).to eq("100#{spec[:mode]}")
          else
            expect(File.stat(unzipped_file_path).mode.to_s(8)).to eq("100#{BitsService::Routes::AppStash::DEFAULT_FILE_MODE.to_s(8)}")
          end
        end

        expect(count_files_in_dir(unzip_path)).to eq(resources.size)
      end
    end

    context 'a single file' do
      let(:resources) do
        [
          { fn: 'init.rb', sha1: app_rb_sha }
        ]
      end

      it_behaves_like 'bundles endpoint'
    end

    context 'multiple files' do
      let(:resources) do
        [
          { fn: 'init.rb', sha1: app_rb_sha },
          { fn: 'lib.rb', sha1: lib_rb_sha, mode: '777' },
          { fn: 'another_one.rb', sha1: lib_rb_sha, mode: '700' }
        ]
      end

      it_behaves_like 'bundles endpoint'
    end

    context 'folders and subfolders' do
      let(:resources) do
        [
          { fn: 'init.rb', sha1: app_rb_sha },
          { fn: 'some-folder/lib.rb', sha1: lib_rb_sha, mode: '600' },
          { fn: 'another-folder/inside/another_one.rb', sha1: lib_rb_sha, mode: '766' },
          { fn: 'some-folder/subfolder/app.rb', sha1: app_rb_sha, mode: '677' }
        ]
      end

      it_behaves_like 'bundles endpoint'
    end

    context 'when the request is for non-existing resources' do
      let(:request_body) { (existing_resources + nonexisting_resources).to_json }

      it 'returns HTTP status 404' do
        expect(response.code).to eq(404)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to eq('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa not found')
      end
    end

    context 'when the request body is empty' do
      let(:request_body) { nil }

      it 'returns HTTP status 400' do
        expect(response.code).to eq(400)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/Request invalid due to parse error/)
      end
    end

    context 'when the request body is invalid json' do
      let(:request_body) { 'invalid' }

      it 'returns HTTP status 400' do
        expect(response.code).to eq(400)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/Request invalid due to parse error/)
      end
    end

    context 'when the json in the request is not an array' do
      let(:request_body) { '{}' }

      it 'returns HTTP status 422' do
        expect(response.code).to eq(422)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The request is semantically invalid: must be a non-empty array/)
      end
    end

    context 'when the json in the request is an empty array' do
      let(:request_body) { '[]' }

      it 'returns HTTP status 422' do
        expect(response.code).to eq(422)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The request is semantically invalid: must be a non-empty array/)
      end
    end

    context 'when the json in the request contains a resource without a SHA' do
      let(:request_body) { '[{"fn":"/app/lib.rb"}]' }

      it 'returns HTTP status 422' do
        expect(response.code).to eq(422)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The request is semantically invalid: key `sha1` missing or empty/)
      end
    end

    context 'when the json in the request contains a resource without a file path' do
      let(:request_body) { '[{"sha1":"zyzfh"}]' }

      it 'returns HTTP status 422' do
        expect(response.code).to eq(422)
      end

      it 'returns a corresponding error' do
        description = JSON.parse(response.body)['description']
        expect(description).to match(/The request is semantically invalid: key `fn` missing or empty/)
      end
    end
  end
end
