require 'json'
require 'bits_service/services/body_builder'

module BitsService
  class NullUpdater
    def processing_upload(*_)
    end

    def failed(*_)
    end

    def ready(*_)
    end
  end

  class CCUpdater
    UpdateError = Class.new(StandardError)
    ResourceNotFoundError = Class.new(StandardError)

    def initialize(cc_url, client)
      @cc_url = cc_url
      @client = client
    end

    def processing_upload(target_guid)
      update_cloud_controller(target_guid, :processing_upload)
    end

    def failed(target_guid, error)
      update_cloud_controller(target_guid, :failed, error: error)
    end

    def ready(target_guid, sha1:, sha256:)
      update_cloud_controller(
        target_guid,
        :ready,
        sha1: sha1,
        sha256: sha256
      )
    end

    private

    def update_cloud_controller(target_guid, state, sha1: nil, sha256: nil, error: nil)
      body = BodyBuilder.new(state, sha1: sha1, sha256: sha256, error: error)

      response = @client.patch("#{@cc_url}/packages/#{target_guid}", body.to_json)

      case response.code
      when 204
        nil # no news are good news
      when 404
        raise ResourceNotFoundError.new(response.content)
      when 422
        raise UpdateError.new(response.content)
      else
        raise "Unexpected response with code #{response.code} from CC: #{response.content}"
      end
    end
  end
end
