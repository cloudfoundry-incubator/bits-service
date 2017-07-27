require 'yaml'

module BitsService
  module Environment
    class << self
      def init
        load_configuration(ENV.fetch('BITS_CONFIG_FILE'))
        initialize_logger
      rescue KeyError => e
        puts 'Missing configuration file.'
        puts 'Please set BITS_CONFIG_FILE to point to a valid configuration file.'
        raise e
      end

      def load_configuration(path)
        @config = YAML.load_file(path).deep_symbolize_keys
      end

      attr_reader :config, :logger

      private

      def initialize_logger
        config = Steno::Config.from_hash(@config[:logging])
        Steno.init(config)
        @logger = Steno.logger('bits')
      end
    end
  end
end
