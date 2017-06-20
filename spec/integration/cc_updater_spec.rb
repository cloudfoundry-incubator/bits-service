require 'spec_helper'
require 'stub_server'

module BitsService
  describe CCUpdater, type: :integration do
    # this is read by Helpers::MTLSHelper
    let(:config) do
      {
        cc_updates: {
          ca_cert: 'spec/certificates/ca.crt',
          client_cert: 'spec/certificates/bits-service.crt',
          client_key: 'spec/certificates/bits-service.key',
        }
      }
    end
    include Helpers::MTLSHelper

    subject { CCUpdater.new("https://localhost:#{port}", mtls_client) }
    let(:port) { 9123 }
    let(:guid) { SecureRandom.uuid }

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

    let(:replies) do
      { "/packages/#{guid}" => [status_code, {}, []] }
    end

    around(:example) do |example|
      StubServer.open(port, replies, ssl: ssl, webrick: webrick_additional_config) do |server|
        server.wait
        example.run
      end
    end

    context 'CC accepts the update' do
      let(:status_code) { 204 }

      it "sets the package state in CC to 'processing upload'" do
        expect { subject.processing_upload(guid) }.to_not raise_error
      end

      it "sets the package state in CC to 'ready'" do
        expect { subject.ready(guid, sha1: 'ignored', sha256: 'ignored') }.to_not raise_error
      end

      it "sets the package state in CC to 'failed'" do
        expect { subject.failed(guid, error: 'ignored') }.to_not raise_error
      end
    end

    context 'CC rejects the update' do
      let(:status_code) { 422 }

      it "sets the package state in CC to 'processing upload'" do
        expect { subject.processing_upload(guid) }.to raise_error CCUpdater::UpdateError
      end

      it "sets the package state in CC to 'ready'" do
        expect { subject.ready(guid, sha1: 'ignored', sha256: 'ignored') }.to raise_error CCUpdater::UpdateError
      end

      it "sets the package state in CC to 'failed'" do
        expect { subject.failed(guid, error: 'ignored') }.to raise_error CCUpdater::UpdateError
      end
    end

    context 'CC responds with unknown status' do
      let(:status_code) { 500 }

      it "sets the package state in CC to 'processing upload'" do
        expect { subject.processing_upload(guid) }.to raise_error StandardError
      end

      it "sets the package state in CC to 'ready'" do
        expect { subject.ready(guid, sha1: 'ignored', sha256: 'ignored') }.to raise_error StandardError
      end

      it "sets the package state in CC to 'failed'" do
        expect { subject.failed(guid, error: 'ignored') }.to raise_error StandardError
      end
    end

    context 'CC responds with ResourceNotFound' do
      let(:status_code) { 404 }

      it "sets the package state in CC to 'processing upload'" do
        expect { subject.processing_upload(guid) }.to raise_error CCUpdater::ResourceNotFoundError
      end

      it "sets the package state in CC to 'ready'" do
        expect { subject.ready(guid, sha1: 'ignored', sha256: 'ignored') }.to raise_error CCUpdater::ResourceNotFoundError
      end

      it "sets the package state in CC to 'failed'" do
        expect { subject.failed(guid, error: 'ignored') }.to raise_error CCUpdater::ResourceNotFoundError
      end
    end
  end
end
