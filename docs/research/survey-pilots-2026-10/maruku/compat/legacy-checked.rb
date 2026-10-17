# The checker build of legacy.rb: Ruby with frozen string literals and deprecation warnings on.
exec "ruby", "--enable=frozen-string-literal", "-W:deprecated", File.join(__dir__, "legacy.rb"), *ARGV
