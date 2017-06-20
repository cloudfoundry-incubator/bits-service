require 'spec_helper'

module BitsService
  module Helpers
    describe CCUpdaterFactory do
      class Dummy
        include CCUpdaterFactory
      end

      let(:config) { nil }

      shared_examples 'CCUpdater' do
        it 'returns a new CCUpdater' do
          cc_updater = Dummy.new.produce_cc_updater(config, nil)
          expect(cc_updater).to be
          expect(cc_updater).to respond_to(:processing_upload)
          expect(cc_updater).to respond_to(:ready)
          expect(cc_updater).to respond_to(:failed)
        end
      end

      it 'understands produce_cc_updater' do
        expect(Dummy.new).to respond_to(:produce_cc_updater)
      end

      context 'without config' do
        let(:config) { nil }
        it_behaves_like 'CCUpdater'
      end

      context 'with a valid config' do
        let(:config) { { cc_url: 'somewhere' } }
        it_behaves_like 'CCUpdater'
      end

      it 'with incomplete config' do
        expect { Dummy.new.produce_cc_updater({}, nil) }.to raise_error StandardError
      end
    end
  end
end
