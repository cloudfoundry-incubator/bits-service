module BitsService
  module Helpers
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
