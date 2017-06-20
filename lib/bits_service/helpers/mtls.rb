require 'httpclient'

module BitsService
  module Helpers
    module MTLSHelper
      def mtls_client
        @client ||= HTTPClient.new.tap do |client|
          if !config[:cc_updates].nil?
            cc_config = config[:cc_updates]
            raise 'Missing ca cert for CC updates' if cc_config[:ca_cert].to_s.empty?
            raise 'Missing client cert for CC updates' if cc_config[:client_cert].to_s.empty?
            raise 'Missing client key for CC updates' if cc_config[:client_key].to_s.empty?

            client.ssl_config.clear_cert_store
            client.ssl_config.add_trust_ca(config[:cc_updates][:ca_cert])
            client.ssl_config.set_client_cert_file(
              config[:cc_updates][:client_cert],
              config[:cc_updates][:client_key]
            )
            client.ssl_config.verify_mode = OpenSSL::SSL::VERIFY_PEER
          end
        end
      end
    end
  end
end
