require_relative './base'

module BitsService
  module Routes
    class Buildpacks < Base
      put '/buildpacks/:guid' do |guid|
        begin
          uploaded_filepath = upload_params.upload_filepath('buildpack')
          fail Errors::ApiError.new_from_details('BuildpackBitsUploadInvalid', 'a file must be provided') if uploaded_filepath.to_s == ''

          statsd.time 'buildpack-cp_to_blobstore-time' do
            buildpack_blobstore.cp_to_blobstore(uploaded_filepath, guid)
          end

          status 201
        ensure
          FileUtils.rm_f(uploaded_filepath) if uploaded_filepath
        end
      end

      get '/buildpacks/:guid' do |guid|
        blob = buildpack_blobstore.blob(guid)
        fail Errors::ApiError.new_from_details('ResourceNotFound', guid) unless blob

        if buildpack_blobstore.local?
          if use_nginx?
            return [200, { 'X-Accel-Redirect' => blob.internal_download_url }, nil]
          else
            return send_file blob.local_path
          end
        else
          return [302, { 'Location' => blob.public_download_url }, nil]
        end
      end

      delete '/buildpacks/:guid' do |guid|
        blob = buildpack_blobstore.blob(guid)
        fail Errors::ApiError.new_from_details('ResourceNotFound', guid) unless blob
        buildpack_blobstore.delete_blob(blob)
        status 204
      end
    end
  end
end
