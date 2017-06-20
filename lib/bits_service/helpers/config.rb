require 'yaml'

module BitsService
  module Helpers
    module CCUpdaterFactory
      def produce_cc_updater(config, mtls_client)
        if config.nil?
          NullUpdater.new
        else
          url = config[:cc_url]

          if url.nil?
            raise 'Missing URL for CC updates'
          else
            CCUpdater.new(url, mtls_client)
          end
        end
      end
    end

    module Config
      def config
        BitsService::Environment.config
      end

      def logger
        BitsService::Environment.logger
      end

      def use_nginx?
        config[:nginx][:use_nginx]
      end

      def public_endpoint
        fail 'no public endpoint configured' unless config[:public_endpoint]
        config[:public_endpoint]
      end
    end
  end
end
