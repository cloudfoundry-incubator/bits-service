require 'spec_helper'
require 'rspec/json_expectations'

module BitsService
  describe CCUpdater do
    subject(:cc_updater) { CCUpdater.new(cc_url, mtls_client) }
    let(:cc_url) { 'https://cc.example.com:1234/internal/v8' }
    let(:mtls_client) { double(HTTPClient) }
    let(:response) { instance_double(HTTP::Message) }

    context 'on a happy day' do
      before do
        allow(response).to receive(:code).and_return(204)
        allow(response).to receive(:content).and_return('happy')
      end

      it 'can update the CC with PROCESSING_UPLOAD' do
        expect(mtls_client).to receive(:patch) do |url, body|
          expect(url).to eq('https://cc.example.com:1234/internal/v8/packages/dummy-guid')
          expect(body).to include_json(state: 'PROCESSING_UPLOAD')
        end.and_return(response)

        cc_updater.processing_upload('dummy-guid')
      end

      it 'can update the CC with READY' do
        expect(mtls_client).to receive(:patch) do |url, body|
          expect(url).to eq('https://cc.example.com:1234/internal/v8/packages/dummy-guid')
          expect(body).to include_json(
            state: 'READY',
            checksums: [
              { type: 'sha1', value: 'potato' },
              { type: 'sha256', value: 'potatoest' }
            ]
          )
        end.and_return(response)

        cc_updater.ready('dummy-guid', sha1: 'potato', sha256: 'potatoest')
      end

      it 'can update the CC with FAILED' do
        expect(mtls_client).to receive(:patch) do |url, body|
          expect(url).to eq('https://cc.example.com:1234/internal/v8/packages/dummy-guid')
          expect(body).to include_json(state: 'FAILED', error: 'some-error')
        end.and_return(response)

        cc_updater.failed('dummy-guid', 'some-error')
      end
    end

    context 'CC rejects an update' do
      before do
        allow(response).to receive(:code).and_return(422)
        allow(response).to receive(:content).and_return('nope')
        expect(mtls_client).to receive(:patch).and_return(response)
      end

      it 'sending PROCESSING_UPLOAD raises an error' do
        expect { cc_updater.processing_upload('dummy-guid') }.to raise_error(BitsService::CCUpdater::UpdateError, 'nope')
      end

      it 'sending READY raises an error' do
        expect { cc_updater.ready('dummy-guid', sha1: 'potato', sha256: 'potatoest') }.to raise_error(BitsService::CCUpdater::UpdateError, 'nope')
      end

      it 'sending FAILED raises an error' do
        expect { cc_updater.failed('dummy-guid', 'another-error') }.to raise_error(BitsService::CCUpdater::UpdateError, 'nope')
      end
    end

    context 'when CC replies with an unexpected response' do
      before do
        allow(response).to receive(:code).and_return(815)
        allow(response).to receive(:content).and_return('Too fast')
        expect(mtls_client).to receive(:patch).and_return(response)
      end

      it 'sending PROCESSING_UPLOAD raises a generic error' do
        expect { cc_updater.processing_upload('dummy-guid') }.to raise_error(StandardError, /Too fast/)
      end
      it 'sending READY raises a generic error' do
        expect { cc_updater.ready('dummy-guid', sha1: 'potato', sha256: 'potatoest') }.to raise_error(StandardError, /Too fast/)
      end
      it 'sending FAILED raises a generic error' do
        expect { cc_updater.failed('dummy-guid', 'another-error') }.to raise_error(StandardError, /Too fast/)
      end
    end

    context 'when CC replies with 404' do
      before do
        allow(response).to receive(:code).and_return(404)
        allow(response).to receive(:content).and_return('ResourceNotFound')
        expect(mtls_client).to receive(:patch).and_return(response)
      end

      it 'sending PROCESSING_UPLOAD raises a ResourceNotFoundError' do
        expect { cc_updater.processing_upload('dummy-guid') }.to raise_error(CCUpdater::ResourceNotFoundError)
      end
      it 'sending READY raises a ResourceNotFoundError' do
        expect { cc_updater.ready('dummy-guid', sha1: 'potato', sha256: 'potatoest') }.to raise_error(CCUpdater::ResourceNotFoundError)
      end
      it 'sending FAILED raises a ResourceNotFoundError' do
        expect { cc_updater.failed('dummy-guid', 'another-error') }.to raise_error(CCUpdater::ResourceNotFoundError)
      end
    end
  end
end
