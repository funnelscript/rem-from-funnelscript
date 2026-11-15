// REM browser page script (adapted from RES)
window.__rem = window.__rem || {};
window.__re = window.__rem;
// annotate.js: numbered outlines on page regions for anatomy screenshots.
// Paste the whole file into the browser's JavaScript tool, then:
//   window.__rem.annotate(['header', '.toolbar', { selector: '.card', label: 'Card' }])
//   window.__rem.annotate()        outlines visible landmarks (header, nav, main, toolbars, dialogs)
//   window.__rem.annotate.clear()
// The overlay uses viewport coordinates: take the screenshot before scrolling.
// Playwright browser_evaluate: wrap as () => { <file>; return window.__rem.last; }
(() => {
  const re = (window.__rem = window.__rem || {});
  const LAYER_ID = '__re-annotate';
  const LANDMARKS = ['header', 'nav', 'main', 'aside', 'footer', '[role="toolbar"]', '[role="tablist"]', '[role="dialog"]', '[role="search"]'];

  function clear() {
    document.getElementById(LAYER_ID)?.remove();
    re.last = { cleared: true };
    return re.last;
  }

  function annotate(targets, options = {}) {
    clear();
    const color = options.color || '#e5007d';
    const list = (targets && targets.length ? targets : LANDMARKS).map((target) =>
      typeof target === 'string' ? { selector: target } : target,
    );
    const layer = document.createElement('div');
    layer.id = LAYER_ID;
    layer.style.cssText = 'position:fixed;inset:0;pointer-events:none;z-index:2147483647;';

    const result = [];
    let n = 0;
    for (const { selector, label } of list) {
      const matches = options.all ? [...document.querySelectorAll(selector)] : [document.querySelector(selector)].filter(Boolean);
      const visible = matches.filter((element) => {
        const rect = element.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && rect.bottom > 0 && rect.top < innerHeight;
      });
      if (!visible.length) {
        result.push({ n: null, selector, label: label || null, found: false });
        continue;
      }
      for (const element of visible) {
        n += 1;
        const rect = element.getBoundingClientRect();
        const box = document.createElement('div');
        box.style.cssText = `position:absolute;left:${rect.left}px;top:${rect.top}px;width:${rect.width}px;height:${rect.height}px;` +
          `outline:2px solid ${color};outline-offset:-1px;`;
        const badge = document.createElement('div');
        badge.textContent = String(n);
        badge.style.cssText = `position:absolute;left:${Math.max(rect.left, 0) + 2}px;top:${Math.max(rect.top, 0) + 2}px;` +
          `min-width:18px;height:18px;padding:0 4px;border-radius:9px;background:${color};color:#fff;` +
          'font:600 12px/18px system-ui,sans-serif;text-align:center;';
        layer.append(box, badge);
        result.push({
          n,
          selector,
          label: label || null,
          found: true,
          box: { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) },
        });
      }
    }
    document.body.appendChild(layer);
    re.last = result;
    return result;
  }

  annotate.clear = clear;
  re.annotate = annotate;
  re.last = { ready: true, usage: "__rem.annotate(['header', '.toolbar']) then __rem.annotate.clear()" };
  return re.last;
})();
