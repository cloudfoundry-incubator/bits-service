require 'spec_helper'

module BitsService
  module Helpers
    describe MTLSHelper do
      class MTLSWrapper
        include MTLSHelper

        def initialize(config)
          @config = config
        end

        private

        attr_reader :config
      end

      shared_examples 'MTLSHelper' do
        it 'returns a new HTTPClient' do
          client = MTLSWrapper.new(config).mtls_client
          expect(client).to respond_to(:patch)
        end
      end

      it 'understands mtls_client' do
        expect(MTLSWrapper.new(nil)).to respond_to(:mtls_client)
      end

      context 'without config' do
        let(:config) { {} }
        it_behaves_like 'MTLSHelper'
      end

      context 'with a valid config' do
        let(:config) {{
            cc_updates: {
              ca_cert: File.expand_path('../../../certificates/ca.crt', __FILE__),
              client_cert: File.expand_path('../../../certificates/server.crt', __FILE__),
              client_key: File.expand_path('../../../certificates/server.key', __FILE__)
          }
        }}
        it_behaves_like 'MTLSHelper'

        it 'returns a new HTTPClient with certs' do
          client = MTLSWrapper.new(config).mtls_client

          expect(client.ssl_config.client_cert).to be
          expect(client.ssl_config.client_key).to be
        end
      end

      context 'with incomplete config' do
        let(:config) { { cc_updates: {} } }
        it 'raises an error' do
          expect { MTLSWrapper.new(config).mtls_client }.to raise_error StandardError
        end
      end
    end
  end
end
