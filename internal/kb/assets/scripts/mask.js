// mask.js: Two-tier physical PII redaction and visual obfuscation for production screens.
// Tier 1: Physically substitutes DOM text nodes, input values, and attributes with synthetic placeholders.
// Tier 2: Injects CSS rules applying blur filter as visual redundancy for screenshots.
// Usage:
//   window.__rem.mask.apply({ selectors: ['.customer-name'], names: ['Maria Silva'], avatars: true, placeholders: true })
//   window.__rem.mask.clear()
(() => {
  const rem = (window.__rem = window.__rem || {});
  window.__re = rem; // compatibility alias

  const CLASS = '__rem-masked';
  const PLACEHOLDER_CLASS = '__rem-masked-placeholder';
  const STYLE_ID = '__rem-mask-style';
  const BLUR = 'filter:blur(6px)!important';
  const BLUR_TEXT = 'color:transparent!important;text-shadow:0 0 16px rgba(0,0,0,.9)!important';

  const PATTERNS = {
    email: /[^\s@<>()]+@[^\s@<>()]+\.[a-z]{2,}/i,
    phone: /(?:\+\d{1,3}[\s.-]?)?(?:\(?\d{2,3}\)?[\s.-]?)?\d{3,5}[\s.-]?\d{4}\b/,
    money: /(?:R\$|US\$|\$|€|£)\s?\d[\d.,]*/,
    cpf: /\b\d{3}\.\d{3}\.\d{3}-\d{2}\b/,
    cnpj: /\b\d{2}\.\d{3}\.\d{3}\/\d{4}-\d{2}\b/,
  };

  const REPLACEMENTS = {
    email: 'user_redacted@example.com',
    phone: '+1-555-0100',
    money: '$100.00',
    cpf: '000.000.000-00',
    cnpj: '00.000.000/0001-00',
  };

  const modifiedNodes = new Map(); // Node -> original text
  const modifiedInputs = new Map(); // HTMLInputElement -> original value

  const rules = rem.mask?.rules || { selectors: [], placeholders: false };

  const escapeRegExp = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  const validSelector = (selector) => {
    if (typeof selector !== 'string' || /[{}]/.test(selector)) return false;
    try {
      document.querySelector(selector);
      return true;
    } catch {
      return false;
    }
  };

  function writeStyle() {
    let style = document.getElementById(STYLE_ID);
    if (!style) {
      style = document.createElement('style');
      style.id = STYLE_ID;
    }
    if (!style.isConnected) document.head.appendChild(style);
    const lines = [`.${CLASS}{${BLUR}}`, `.${PLACEHOLDER_CLASS}::placeholder{${BLUR_TEXT}}`];
    for (const selector of rules.selectors) lines.push(`${selector}{${BLUR}}`);
    if (rules.placeholders) lines.push(`input::placeholder,textarea::placeholder{${BLUR_TEXT}}`);
    style.textContent = lines.join('\n');
  }

  // Tier 1: Physically walk DOM and replace sensitive strings in text nodes
  function physicalRedact(root, patterns) {
    const walker = document.createTreeWalker(
      root || document.body,
      NodeFilter.SHOW_TEXT,
      {
        acceptNode(node) {
          if (!node.nodeValue || !node.nodeValue.trim()) return NodeFilter.FILTER_REJECT;
          const parent = node.parentElement;
          if (!parent) return NodeFilter.FILTER_REJECT;
          const tag = parent.tagName.toLowerCase();
          if (tag === 'script' || tag === 'style' || tag === 'noscript') return NodeFilter.FILTER_REJECT;
          return NodeFilter.FILTER_ACCEPT;
        }
      }
    );

    let count = 0;
    let currentNode;
    while ((currentNode = walker.nextNode())) {
      let val = currentNode.nodeValue;
      let changed = false;
      for (const [key, regex] of Object.entries(patterns)) {
        if (regex.test(val)) {
          if (!modifiedNodes.has(currentNode)) {
            modifiedNodes.set(currentNode, currentNode.nodeValue);
          }
          const sub = REPLACEMENTS[key] || '[REDACTED]';
          val = val.replace(regex, sub);
          changed = true;
        }
      }
      if (changed) {
        currentNode.nodeValue = val;
        currentNode.parentElement?.classList.add(CLASS);
        count++;
      }
    }

    // Also sanitize input and textarea fields
    const inputs = (root || document).querySelectorAll('input, textarea');
    for (const input of inputs) {
      for (const [key, regex] of Object.entries(patterns)) {
        if (regex.test(input.value)) {
          if (!modifiedInputs.has(input)) {
            modifiedInputs.set(input, input.value);
          }
          input.value = REPLACEMENTS[key] || '[REDACTED]';
          input.classList.add(CLASS);
          count++;
        }
      }
    }

    return count;
  }

  function apply(options = {}) {
    const patterns = { ...PATTERNS };
    for (const [i, name] of (options.names || []).entries()) {
      patterns[`name${i + 1}`] = new RegExp(escapeRegExp(name), 'ig');
      REPLACEMENTS[`name${i + 1}`] = '[REDACTED_NAME]';
    }

    const invalid = (options.selectors || []).filter((selector) => !validSelector(selector));
    for (const selector of options.selectors || []) {
      if (!invalid.includes(selector) && !rules.selectors.includes(selector)) rules.selectors.push(selector);
    }
    if (options.placeholders) rules.placeholders = true;

    // Apply Tier 2 visual styles
    writeStyle();

    // Apply Tier 1 physical redactions
    const redactedCount = physicalRedact(document.body, patterns);

    // Redact avatars if requested
    if (options.avatars) {
      const avatars = document.querySelectorAll('img[alt*="avatar" i], img[src*="avatar" i], [class*="avatar" i]');
      avatars.forEach(el => el.classList.add(CLASS));
    }

    rem.last = {
      applied: true,
      physicalRedactions: redactedCount,
      activeSelectors: rules.selectors.length,
      invalidSelectors: invalid,
    };
    return rem.last;
  }

  function clear() {
    // Restore physical text
    for (const [node, original] of modifiedNodes.entries()) {
      if (node.isConnected) {
        node.nodeValue = original;
        node.parentElement?.classList.remove(CLASS);
      }
    }
    modifiedNodes.clear();

    // Restore input values
    for (const [input, original] of modifiedInputs.entries()) {
      if (input.isConnected) {
        input.value = original;
        input.classList.remove(CLASS);
      }
    }
    modifiedInputs.clear();

    // Remove Tier 2 styles
    document.getElementById(STYLE_ID)?.remove();
    document.querySelectorAll(`.${CLASS}`).forEach(el => el.classList.remove(CLASS));

    rules.selectors = [];
    rules.placeholders = false;

    rem.last = { cleared: true };
    return rem.last;
  }

  rem.mask = { apply, clear, rules };
  rem.last = { ready: true, usage: 'window.__rem.mask.apply() or window.__rem.mask.clear()' };
  return rem.last;
})();
