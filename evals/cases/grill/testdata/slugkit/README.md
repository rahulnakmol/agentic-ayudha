# slugkit

Synthetic eval fixture: a small JavaScript library that turns titles into URL slugs.

```js
const { createSlugger } = require("slugkit");
const slugger = createSlugger({
  separator: "-", lowercase: true, maxLength: 0, locale: "en", transliterate: true,
});
slugger.slug("Crème Brûlée Recipe"); // "creme-brulee-recipe"
```

`createSlugger` throws `invalid options` unless all five options are given.
