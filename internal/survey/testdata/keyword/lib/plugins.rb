# require "plugins/#{legacy}" was the old way
names.each { |e| require "plugins/#{e}" }
require 'json'
