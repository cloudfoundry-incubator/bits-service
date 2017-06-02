require 'spec_helper'
require 'rspec/expectations'

RSpec::Matchers.define :have_status_ok_and_body do |expected|
  match { |actual| actual.ok? == true && actual.body == expected }
end

module BitsService
  module Routes
    describe Sign do
      around(:each) do |example|
        config_filepath = create_config_file({ public_endpoint: 'http://bits-service.example.com' })
        BitsService::Environment.load_configuration(config_filepath)

        example.run

        FileUtils.rm_f(config_filepath)
      end

      let(:blobstore) { double(Blobstore::Client) }

      before do
        allow_any_instance_of(Routes::Sign).to receive(:packages_blobstore).and_return(blobstore)
        allow_any_instance_of(Routes::Sign).to receive(:droplet_blobstore).and_return(blobstore)
        allow_any_instance_of(Routes::Sign).to receive(:buildpack_blobstore).and_return(blobstore)
        allow_any_instance_of(Routes::Sign).to receive(:buildpack_cache_blobstore).and_return(blobstore)
      end

      describe 'GET /sign/.*' do
        let(:time_of_request) { Time.new(2016, 1, 1, 0, 0, 0, '+00:00') }
        let(:signer) { double }
        context 'blobstore is not local' do
          before do
            allow(blobstore).to receive(:local?).and_return(false)
            allow(Time).to receive(:now).and_return(time_of_request)

            allow_any_instance_of(Routes::Sign).to receive(:signer).and_return(signer)
            allow(signer).to receive(:sign).and_return 'some_md5_sum'
          end

          it 'returns the blob\'s public package download url' do
            allow(blobstore).to receive(:public_download_url).with('bar').and_return('http://blobstore.example.com/a-signed-package-url')
            get '/sign/packages/bar'
            expect(last_response).to have_status_ok_and_body 'http://blobstore.example.com/a-signed-package-url'
          end

          it "returns the blob's public package download url" do
            allow(blobstore).to receive(:public_download_url).with('foo').and_return('http://blobstore.example.com/a-signed-package-url-for-get')
            get '/sign/packages/foo?verb=get'
            expect(last_response).to have_status_ok_and_body 'http://blobstore.example.com/a-signed-package-url-for-get'
          end

          it 'returns the blob\'s public droplet download url' do
            allow(blobstore).to receive(:public_download_url).with('1234/5678').and_return('http://blobstore.example.com/a-signed-droplet-url')
            get '/sign/droplets/1234/5678'
            expect(last_response).to have_status_ok_and_body 'http://blobstore.example.com/a-signed-droplet-url'
          end

          it 'returns the blob\'s public buildpack download url' do
            allow(blobstore).to receive(:public_download_url).with('foo').and_return('http://blobstore.example.com/a-signed-buildpack-url')
            get '/sign/buildpacks/foo'
            expect(last_response).to have_status_ok_and_body 'http://blobstore.example.com/a-signed-buildpack-url'
          end

          it 'returns the blob\'s public buildpack_cache entry download url' do
            allow(blobstore).to receive(:public_download_url).with('some_guid/some_file').and_return('http://blobstore.example.com/a-signed-buildpack_cache-url')
            get '/sign/buildpack_cache/entries/some_guid/some_file'
            expect(last_response).to have_status_ok_and_body 'http://blobstore.example.com/a-signed-buildpack_cache-url'
          end

          it "returns the blob's public package upload url" do
            allow(blobstore).to receive(:public_upload_url).with('bar')
            get '/sign/packages/bar?verb=put'
            expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/packages/bar?md5=some_md5_sum&expires=1451610000'
          end

          it "returns the blob's public droplet upload url" do
            allow(blobstore).to receive(:public_upload_url).with('bar')
            get '/sign/droplets/1234/5678?verb=put'
            expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/droplets/1234/5678?md5=some_md5_sum&expires=1451610000'
          end

          it "returns the blob's public buildpack upload url" do
            allow(blobstore).to receive(:public_upload_url).with('bar')
            get '/sign/buildpacks/foo?verb=put'
            expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpacks/foo?md5=some_md5_sum&expires=1451610000'
          end

          it "returns the blob's public buildpack_cache upload url" do
            allow(blobstore).to receive(:public_upload_url).with('bar')
            get '/sign/buildpack_cache/entries/some_guid/some_file?verb=put'
            expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpack_cache/entries/some_guid/some_file?md5=some_md5_sum&expires=1451610000'
          end
        end

        context 'blobstore is local' do
          let(:time_of_request) { Time.new(2016, 1, 1, 0, 0, 0, '+00:00') }
          let(:signer) { double }

          before do
            allow(blobstore).to receive(:local?).and_return(true)
            allow(Time).to receive(:now).and_return(time_of_request)

            allow_any_instance_of(Routes::Sign).to receive(:signer).and_return(signer)
            allow(signer).to receive(:sign).and_return 'some_md5_sum'
          end

          context 'and queries for download url' do
            it 'returns a generated package URL signed by the signer' do
              get '/sign/packages/bar'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/packages/bar?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated droplet URL signed by the signer' do
              get '/sign/droplets/1234/5678'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/droplets/1234/5678?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated buildpack URL signed by the signer' do
              get '/sign/buildpacks/foo'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpacks/foo?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated buildpack_cache URL signed by the signer' do
              get '/sign/buildpack_cache/entries/foo/bar'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpack_cache/entries/foo/bar?md5=some_md5_sum&expires=1451610000'
            end
          end

          context 'and queries for upload url' do
            it "returns a generated packages URL signed by the signer'" do
              get '/sign/packages/bar?verb=put'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/packages/bar?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated droplet URL signed by the signer' do
              get '/sign/droplets/1234/5678?verb=put'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/droplets/1234/5678?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated buildpack URL signed by the signer' do
              get '/sign/buildpacks/foo?verb=put'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpacks/foo?md5=some_md5_sum&expires=1451610000'
            end

            it 'returns a generated buildpack_cache URL signed by the signer' do
              get '/sign/buildpack_cache/entries/foo/bar?verb=put'
              expect(last_response).to have_status_ok_and_body 'http://bits-service.example.com/signed/buildpack_cache/entries/foo/bar?md5=some_md5_sum&expires=1451610000'
            end
          end
        end
      end
    end
  end
end
