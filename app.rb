require 'bundler'
Bundler.require

$LOAD_PATH.unshift File.join(File.dirname(__FILE__), 'lib')

require 'active_support/core_ext/object/try'
require 'active_support/core_ext/hash'

require 'bits_service'

BitsService::Environment.init

helpers BitsService::Helpers::Config
helpers BitsService::Helpers::Upload
helpers BitsService::Helpers::Blobstore
helpers BitsService::Helpers::StatsdHelper
helpers BitsService::Helpers::MTLSHelper

set :dump_errors, false if ENV['RACK_ENV'] == 'production'

module BitsService
  class App < Sinatra::Application
    use Routes::Buildpacks
    use Routes::BuildpackCache
    use Routes::Droplets
    use Routes::AppStash
    use Routes::Packages
    use Routes::Sign
  end
end
