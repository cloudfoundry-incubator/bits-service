require_relative './base'
require 'fileutils'

module BitsService
  module Routes
    class AppStash < Base
      DEFAULT_FILE_MODE = 0744

      post '/app_stash/entries' do
        uploaded_filepath = upload_params.upload_filepath('application')
        fail Errors::ApiError.new_from_details(
          'AppBitsUploadInvalid',
          'missing key `application`') unless uploaded_filepath

        destination_path = Dir.mktmpdir('app_stash')

        begin
          AppPackager.unzip(uploaded_filepath, destination_path)
          statsd.time 'app_stash-cp_r_to_blobstore-time.sparse-avg' do
            app_stash_blobstore.cp_r_to_blobstore(destination_path)
          end
          receipt = Receipt.new(destination_path)

          json 201, receipt.contents
        rescue Errno::ENOSPC
          fail Errors::ApiError.new_from_details('NoSpaceOnDevice')
        ensure
          FileUtils.rm_r(destination_path)
        end
      end

      post '/app_stash/matches' do
        response_payload = request_payload.select do |resource|
          valid_sha?(resource['sha1']) && app_stash_blobstore.exists?(resource['sha1'])
        end

        json 200, response_payload
      end

      post '/app_stash/bundles' do
        destination_path = Dir.mktmpdir('app_stash')

        request_payload.each do |resource|
          validate_resource!(resource)

          sha1 = resource['sha1']
          fn = resource['fn']
          mode = parse_mode!(resource['mode'])
          app_stash_blobstore.download_from_blobstore(sha1, File.join(destination_path, fn), mode: mode)
        end

        output_path = File.join(Dir.mktmpdir('app_stash'), 'bundle.zip')
        AppPackager.zip(destination_path, output_path)

        status 200
        stream do |out|
          begin
            out << File.open(output_path).read
          ensure
            FileUtils.rm_rf(destination_path)
            FileUtils.rm_rf(output_path)
          end
        end
      end

      private

      def parse_mode!(raw_mode)
        (raw_mode ? raw_mode.to_i(8) : DEFAULT_FILE_MODE).tap do |mode|
          raise Errors::ApiError.new_from_details('AppResourcesFileModeInvalid', "File mode '#{raw_mode}' is invalid.") unless (mode & 0600) == 0600
        end
      end

      def valid_sha?(sha)
        sha.to_s.length == 40
      end

      def request_payload
        return @request_payload if @request_payload

        @request_payload = JSON.parse request.body.read
        unless @request_payload.is_a?(Array) && !@request_payload.empty?
          fail Errors::ApiError.new_from_details('UnprocessableEntity', 'must be a non-empty array.')
        end

        @request_payload
      rescue JSON::ParserError => e
        fail Errors::ApiError.new_from_details('MessageParseError', e.message)
      end

      def validate_resource!(resource)
        sha1 = resource['sha1']
        unless sha1 && sha1.to_s != ''
          fail Errors::ApiError.new_from_details('UnprocessableEntity', 'key `sha1` missing or empty')
        end

        fn = resource['fn']
        unless fn && fn.to_s != ''
          fail Errors::ApiError.new_from_details('UnprocessableEntity', 'key `fn` missing or empty')
        end

        unless app_stash_blobstore.exists?(sha1)
          fail Errors::ApiError.new_from_details('ResourceNotFound', "#{sha1} not found")
        end
      end
    end
  end
end
