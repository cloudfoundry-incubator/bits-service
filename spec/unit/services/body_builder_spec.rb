require 'spec_helper'
require 'rspec/json_expectations'

module BitsService
  describe BodyBuilder do
    subject { BodyBuilder.new(state, sha1: sha1, sha256: sha256, error: error).to_json }

    let(:state) { 'drinking_coffee' }
    let(:sha1) { nil }
    let(:sha256) { nil }
    let(:error) { nil }

    context 'with just the state' do
      it 'generates a JSON object that contains nothing but the state' do
        expect(subject).to include_json(state: 'DRINKING_COFFEE')
        expect(subject).to_not include_json(checksums: be)
        expect(subject).to_not include_json(error: be)
      end
    end

    context 'with state and only the sha1 checksum' do
      let(:sha1) { '4711' }

      it 'generates a JSON object that contains the state and the checksum' do
        expect(subject).to include_json(state: 'DRINKING_COFFEE')
        expect(subject).to include_json(
          checksums: [
            {
              type: 'sha1',
              value: '4711'
            }
          ]
        )
        expect(subject).to_not include_json(sha256: be)
      end
    end

    context 'with state and only the sha256 checksum' do
      let(:sha256) { '8470' }

      it 'generates a JSON object that contains the state and the checksum' do
        expect(subject).to include_json(state: 'DRINKING_COFFEE')
        expect(subject).to include_json(
          checksums: [
            {
              type: 'sha256',
              value: '8470'
            }
          ]
        )
        expect(subject).to_not include_json(sha1: be)
      end
    end

    context 'with state and both checksums' do
      let(:sha1) { '4711' }
      let(:sha256) { '8470' }

      it 'generates a JSON object that contains the state and the checksum' do
        expect(subject).to include_json(state: 'DRINKING_COFFEE')
        expect(subject).to include_json(
          checksums: [
            {
              type: 'sha1',
              value: '4711'
            },
            {
              type: 'sha256',
              value: '8470'
            }
          ]
        )
        expect(subject).to_not include_json(error: be)
      end
    end

    context 'with state and only an error' do
      let(:error) { 'Something is rotten in the state of Denmark' }

      it 'generates a JSON object that contains the state and the checksum' do
        expect(subject).to include_json(state: 'DRINKING_COFFEE')
        expect(subject).to include_json(
          error: /Denmark/
        )
        expect(subject).to_not include_json(checksums: be)
      end
    end
  end
end
