require 'json'

module BitsService
  class BodyBuilder
    def initialize(state, sha1: nil, sha256: nil, error: nil)
      @state = state
      @sha1 = sha1
      @sha256 = sha256
      @error = error
    end

    def to_json
      body = {}

      body[:state] = state.to_s.upcase
      body[:checksums] = [] if sha1 || sha256
      body[:checksums] << { type: :sha1, value: sha1 } if sha1
      body[:checksums] << { type: :sha256, value: sha256 } if sha256
      body[:error] = error if error

      body.to_json
    end

    private

    attr_reader :state, :sha1, :sha256, :error
  end
end
