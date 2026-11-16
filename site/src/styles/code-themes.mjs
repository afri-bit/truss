// Syntax colours for the docs' code blocks, from the brand palette (tokens.css). Expressive Code's
// default themes use violet and magenta; these use amber, green, blue and teal and never violet.
// Both are TextMate themes. The block background and chrome come from Starlight's variables, which
// brand.css maps onto the tokens.

const rules = (c) => [
  { scope: ['comment', 'punctuation.definition.comment'], settings: { foreground: c.comment, fontStyle: 'italic' } },
  { scope: ['string', 'string.quoted', 'punctuation.definition.string'], settings: { foreground: c.string } },
  { scope: ['constant.numeric', 'constant.language', 'constant.character', 'support.constant'], settings: { foreground: c.number } },
  { scope: ['keyword', 'keyword.control', 'storage', 'storage.type', 'storage.modifier'], settings: { foreground: c.keyword } },
  { scope: ['keyword.operator', 'punctuation', 'meta.brace', 'punctuation.separator', 'punctuation.definition.block'], settings: { foreground: c.punct } },
  { scope: ['entity.name.function', 'support.function', 'meta.function-call', 'variable.function'], settings: { foreground: c.fn } },
  { scope: ['entity.name.type', 'entity.name.class', 'support.type', 'support.class', 'entity.other.inherited-class'], settings: { foreground: c.type } },
  { scope: ['entity.name.tag', 'meta.tag'], settings: { foreground: c.type } },
  {
    scope: [
      'entity.name.tag.yaml',
      'support.type.property-name',
      'support.type.property-name.json',
      'meta.object-literal.key',
      'variable.other.property',
      'variable.other.object.property',
    ],
    settings: { foreground: c.key },
  },
  { scope: ['variable', 'variable.other', 'variable.parameter', 'meta.definition.variable'], settings: { foreground: c.fg } },
  { scope: ['support.function.builtin.shell', 'support.function.shell', 'entity.name.command.shell'], settings: { foreground: c.fn } },
  { scope: ['constant.other.option', 'string.unquoted.argument', 'variable.parameter.option'], settings: { foreground: c.number } },
  { scope: ['markup.heading', 'entity.name.section'], settings: { foreground: c.keyword, fontStyle: 'bold' } },
  { scope: ['markup.inserted'], settings: { foreground: c.string } },
  { scope: ['markup.deleted', 'invalid', 'invalid.illegal'], settings: { foreground: c.bad } },
  { scope: ['markup.italic'], settings: { fontStyle: 'italic' } },
  { scope: ['markup.bold'], settings: { fontStyle: 'bold' } },
  { scope: ['string.regexp', 'constant.character.escape'], settings: { foreground: c.type } },
];

const make = (name, type, c) => ({
  name,
  type,
  colors: { 'editor.background': c.bg, 'editor.foreground': c.fg },
  fg: c.fg,
  bg: c.bg,
  settings: [{ settings: { foreground: c.fg, background: c.bg } }, ...rules(c)],
});

export const trussDark = make('truss-dark', 'dark', {
  bg: '#13161b',
  fg: '#e5e9ef',
  comment: '#808a97',
  string: '#5ccf8a',
  number: '#d8b24a',
  keyword: '#f0b45a',
  punct: '#aab3bf',
  fn: '#5b9cf0',
  type: '#4fc3c3',
  key: '#8fa6c4',
  bad: '#f0685f',
});

export const trussLight = make('truss-light', 'light', {
  bg: '#f8f9fa',
  fg: '#232932',
  comment: '#5b6470',
  string: '#17703a',
  number: '#7d5d00',
  keyword: '#8a5206',
  punct: '#464e5a',
  fn: '#1a5fb8',
  type: '#0f6e6e',
  key: '#36597f',
  bad: '#c9312b',
});
