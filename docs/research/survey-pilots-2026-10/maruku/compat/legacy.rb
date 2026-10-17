$LOAD_PATH.unshift File.expand_path('lib')
require 'maruku'
$render = ->(md, params) { Maruku.new(md, params).to_html }
require_relative 'harness'
