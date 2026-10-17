# Runs one Maruku block doc (spec/block_docs/*.md) through $render and compares
# its HTML with the doc's "Output of to_html", whitespace between tags
# normalised as Maruku's own spec does with nokogiri-diff.
# Exit 0 pass, 1 differ, 77 skip (PENDING doc, no expected HTML, blahtex absent).
require 'nokogiri'
doc = ARGV.fetch(0)
parts = File.read(doc, encoding: 'UTF-8').split(/\n\*{3}\s*([^*\n]+?)\s*\*{3}\n/m)
comment = parts.shift.to_s.strip
sections = Hash[parts.each_slice(2).map { |k, v| [k.strip, v.to_s] }]
exit 77 if comment.start_with?('PENDING')
exit 77 if doc.include?('blahtex') && !system('which blahtex > /dev/null 2>&1')
expected = sections['Output of to_html'].to_s.strip
exit 77 if expected.empty?
params = sections['Parameters:'] || '{}'
html = $render.call(sections['Markdown input:'].to_s, eval(params))
puts html
def norm(h)
  f = Nokogiri::HTML::DocumentFragment.parse(h)
  f.to_html.gsub(/>\s+</, '><').gsub(/\s+/, ' ').strip
end
exit(norm(html) == norm(expected) ? 0 : 1)
