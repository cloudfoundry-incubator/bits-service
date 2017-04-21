require_relative './base'

module BitsService
  module Routes
    class Droplets < Base
      put %r{^/droplets/(.*/.*)} do |path|
        begin
          uploaded_filepath = upload_params.upload_filepath('droplet')
          return create_from_upload(uploaded_filepath, path) if uploaded_filepath

          source_guid = parsed_body['source_guid']
          return create_as_duplicate(source_guid, path) if source_guid

          fail Errors::ApiError.new_from_details('InvalidDropletSource')
        ensure
          FileUtils.rm_f(uploaded_filepath) if uploaded_filepath
        end
      end

      get %r{^/droplets/(.*/.*)} do |path|
        blob = droplet_blobstore.blob(path)
        fail Errors::ApiError.new_from_details('ResourceNotFound', path) unless blob

        if droplet_blobstore.local?
          if use_nginx?
            return [200, { 'X-Accel-Redirect' => blob.internal_download_url }, nil]
          else
            return send_file blob.local_path
          end
        else
          return [302, { 'Location' => blob.public_download_url }, nil]
        end
      end

      delete %r{^/droplets/(.*/.*)} do |path|
        blob = droplet_blobstore.blob(path)
        fail Errors::ApiError.new_from_details('ResourceNotFound', path) unless blob
        droplet_blobstore.delete_blob(blob)
        status 204
      end

      private

      def create_from_upload(uploaded_filepath, path)
        fail Errors::ApiError.new_from_details('DropletUploadInvalid', 'a file must be provided') if uploaded_filepath.to_s == ''


        statsd.time 'droplet-cp_to_blobstore-time' do
          droplet_blobstore.cp_to_blobstore(uploaded_filepath, path)
        end

        status 201
      ensure
        FileUtils.rm_f(uploaded_filepath) if uploaded_filepath
      end

      def create_as_duplicate(source_guid, new_guid)
        blob = droplet_blobstore.blob(source_guid)
        fail Errors::ApiError.new_from_details('ResourceNotFound', source_guid) unless blob

        droplet_blobstore.cp_file_between_keys(source_guid, new_guid)
        status 201
      end

      def parsed_body
        body = request.body.read

        if body.empty?
          {}
        else
          JSON.parse(body)
        end
      rescue JSON::ParserError => e
        fail Errors::ApiError.new_from_details('MessageParseError', e.message)
      end
    end
  end
end
