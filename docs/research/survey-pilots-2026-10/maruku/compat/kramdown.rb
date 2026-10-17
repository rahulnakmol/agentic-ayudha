require 'kramdown'
$render = ->(md, _params) { Kramdown::Document.new(md).to_html }
require_relative 'harness'
