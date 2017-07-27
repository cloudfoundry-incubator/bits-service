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
  end
end
