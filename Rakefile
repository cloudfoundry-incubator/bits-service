require 'rake'
require 'rake/clean'
require 'rspec/core/rake_task'
require 'rubocop/rake_task'

task default: ['spec:all']

namespace :spec do
  desc 'Run all specs'
  task all: ['rubocop:auto_correct', :unit, :integration]

  RSpec::Core::RakeTask.new(:unit) do |t|
    t.pattern = 'spec/unit/**/*_spec.rb'
  end

  RSpec::Core::RakeTask.new(:integration) do |t|
    t.pattern = 'spec/integration/**/*_spec.rb'
  end
end

RuboCop::RakeTask.new

namespace :docs do
  SOURCE_FILES = FileList['docs/*.txt']
  CLEAN.include SOURCE_FILES.pathmap('%d/%n.png')

  SOURCE_FILES.each do |src|
    target = src.pathmap('%d/%n.png')
    desc "Render #{target} from #{src}"
    file target do
      `docs/websequencediagram #{src} #{target}`
      warn "#{src} rendered to #{target}"
    end
  end
end
