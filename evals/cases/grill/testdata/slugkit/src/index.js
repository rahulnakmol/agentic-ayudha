// slugkit: turn a title into a URL slug. Every option is required.
const REQUIRED = ["separator", "lowercase", "maxLength", "locale", "transliterate"];

function createSlugger(options) {
  for (const key of REQUIRED) {
    if (!options || !(key in options)) {
      throw new Error("invalid options");
    }
  }
  return { slug: (text) => slug(text, options) };
}

function slug(text, { separator, lowercase, maxLength, transliterate }) {
  let s = transliterate ? text.normalize("NFKD").replace(/[\u0300-\u036f]/g, "") : text;
  s = s.replace(/[^A-Za-z0-9]+/g, separator).replace(new RegExp(`^\\${separator}|\\${separator}$`, "g"), "");
  if (lowercase) s = s.toLowerCase();
  return maxLength > 0 ? s.slice(0, maxLength) : s;
}

module.exports = { createSlugger };
