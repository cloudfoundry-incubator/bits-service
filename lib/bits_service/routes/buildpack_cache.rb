require_relative './base'

module BitsService
  module Routes
    class BuildpackCache < Base
      put '/buildpack_cache/entries/:app_guid/:stack_name' do |app_guid, stack_name|
        begin
          uploaded_filepath = upload_params.upload_filepath('buildpack_cache')
          fail Errors::ApiError.new_from_details('BuildpackCacheBitsUploadInvalid', 'a file must be provided') if uploaded_filepath.to_s == ''

          statsd.time 'buildpack_cache-cp_to_blobstore-time.sparse-avg' do
            buildpack_cache_blobstore.cp_to_blobstore(uploaded_filepath, key(app_guid, stack_name))
          end

          status 201
        rescue Errno::ENOSPC
          fail Errors::ApiError.new_from_details('NoSpaceOnDevice')
        ensure
          FileUtils.rm_f(uploaded_filepath) if uploaded_filepath
        end
      end

      get '/buildpack_cache/entries/:app_guid/:stack_name' do |app_guid, stack_name|
        cache_key = key(app_guid, stack_name)
        blob = buildpack_cache_blobstore.blob(cache_key)
        fail Errors::ApiError.new_from_details('ResourceNotFound', cache_key) unless blob

        if buildpack_cache_blobstore.local?
          if use_nginx?
            return [200, { 'X-Accel-Redirect' => blob.internal_download_url }, nil]
          else
            return send_file blob.local_path
          end
        else
          return [302, { 'Location' => blob.public_download_url }, nil]
        end
      end

      delete '/buildpack_cache/entries/:app_guid' do |app_guid|
        buildpack_cache_blobstore.delete_all_in_path(app_guid)
        status 204
      end

      delete '/buildpack_cache/entries/:app_guid/:stack_name' do |app_guid, stack_name|
        cache_key = key(app_guid, stack_name)
        blob = buildpack_cache_blobstore.blob(cache_key)
        fail Errors::ApiError.new_from_details('ResourceNotFound', cache_key) unless blob
        buildpack_cache_blobstore.delete_blob(blob)
        status 204
      end

      delete '/buildpack_cache/entries' do
        buildpack_cache_blobstore.delete_all
        status 204
      end

      private

      def key(app_guid, stack_name)
        "#{app_guid}/#{stack_name}"
      end
    end
  end
end
