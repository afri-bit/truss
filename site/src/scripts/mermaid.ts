// Renders the diagrams the docs write as ```mermaid blocks (scripts/sync.mjs turns them into
// <pre class="mermaid">). Loaded only on pages that have one; draws again when the theme changes.
const blocks = Array.from(document.querySelectorAll<HTMLElement>('pre.mermaid'));

if (blocks.length > 0) {
  const sources = new Map(blocks.map((el) => [el, el.textContent ?? '']));
  const css = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  const draw = async () => {
    const { default: mermaid } = await import('mermaid');
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'base',
      themeVariables: {
        fontFamily: css('--tr-sans'),
        background: css('--tr-surface'),
        primaryColor: css('--tr-surface-3'),
        primaryTextColor: css('--tr-ink'),
        primaryBorderColor: css('--tr-line-2'),
        secondaryColor: css('--tr-accent-soft'),
        tertiaryColor: css('--tr-surface-2'),
        lineColor: css('--tr-text-3'),
        textColor: css('--tr-text'),
        noteBkgColor: css('--tr-accent-soft'),
        noteTextColor: css('--tr-ink'),
        noteBorderColor: css('--tr-accent'),
        clusterBkg: css('--tr-surface-2'),
        clusterBorder: css('--tr-line-2'),
      },
    });
    for (const [el, text] of sources) {
      el.removeAttribute('data-processed');
      el.textContent = text;
    }
    await mermaid.run({ nodes: blocks });
  };

  // One draw at a time, so a quick theme toggle cannot start two over the same nodes.
  let queue: Promise<void> = Promise.resolve();
  const redraw = () => {
    queue = queue.then(draw).catch(() => undefined);
  };
  redraw();
  new MutationObserver(redraw).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  });
}
