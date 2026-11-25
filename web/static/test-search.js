// Suchfeld für Tests: beim Klick auf das Feld alle Tests anbieten, beim Tippen
// filtern, Auswahl mit Maus, Tastatur oder Enter. Kein Framework, keine Inline-Skripte.
(function () {
  const box = document.querySelector('.q-add');
  if (!box) return;
  const input = box.querySelector('input[type="search"]');
  const list = box.querySelector('.suggest');
  if (!input || !list) return;

  const action = box.dataset.action || ('/befund/' + box.dataset.befund + '/test');
  const ALL_LIMIT = 30; // so viele Tests zeigt die Liste ohne Suchbegriff
  let timer = null;
  let seq = 0; // letzte Anfrage gewinnt (verhindert flackernde ältere Antworten)
  let shown = null; // zuletzt angezeigter Suchbegriff
  let active = -1;

  function clear() {
    list.hidden = true;
    list.innerHTML = '';
    shown = null;
    active = -1;
  }

  function items() {
    return Array.from(list.querySelectorAll('.suggest-item'));
  }

  function markActive() {
    items().forEach(function (el, i) {
      if (i === active) {
        el.classList.add('active');
        el.setAttribute('aria-selected', 'true');
        if (el.scrollIntoView) el.scrollIntoView({ block: 'nearest' });
      } else {
        el.classList.remove('active');
        el.setAttribute('aria-selected', 'false');
      }
    });
  }

  function pick(id, title) {
    const form = document.createElement('form');
    form.method = 'post';
    form.action = action;
    const hidden = document.createElement('input');
    hidden.type = 'hidden';
    hidden.name = 'test_id';
    hidden.value = id;
    form.appendChild(hidden);
    document.body.appendChild(form);
    input.value = title;
    form.submit();
  }

  async function search(q) {
    const limit = q ? 15 : ALL_LIMIT;
    const res = await fetch('/api/tests?limit=' + limit + '&q=' + encodeURIComponent(q), {
      headers: { Accept: 'application/json' },
      cache: 'no-store',
    });
    if (!res.ok) throw new Error('Suche fehlgeschlagen');
    return res.json();
  }

  function render(list_, q) {
    if (!list_.length) {
      list.innerHTML = '<li class="muted">kein Treffer</li>';
      list.hidden = false;
      return;
    }
    list.innerHTML = '';
    list_.forEach(function (it) {
      const li = document.createElement('li');
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'suggest-item';
      btn.setAttribute('role', 'option');
      btn.setAttribute('aria-selected', 'false');
      btn.textContent = it.title;
      btn.addEventListener('click', function () { pick(it.id, it.title); });
      li.appendChild(btn);
      list.appendChild(li);
    });
    if (!q) {
      const hint = document.createElement('li');
      hint.className = 'muted suggest-hint';
      hint.textContent = list_.length + ' Tests – tippen zum Filtern';
      list.appendChild(hint);
    }
    list.hidden = false;
  }

  async function load(q, force) {
    if (!force && shown === q) return;
    const mine = ++seq;
    let found;
    try {
      found = await search(q);
    } catch (e) {
      if (mine !== seq) return;
      list.innerHTML = '<li class="muted">Suche nicht möglich</li>';
      list.hidden = false;
      return;
    }
    if (mine !== seq) return; // inzwischen neue Anfrage
    shown = q;
    active = -1;
    render(found, q);
  }

  // Klick oder Fokus ins leere Feld: alle Tests zeigen.
  input.addEventListener('focus', function () { load(input.value.trim(), false); });
  input.addEventListener('click', function () { load(input.value.trim(), false); });

  input.addEventListener('input', function () {
    const q = input.value.trim();
    clearTimeout(timer);
    timer = setTimeout(function () { load(q, false); }, 120);
  });

  input.addEventListener('keydown', function (e) {
    const els = items();
    if (e.key === 'Escape') { clear(); return; }
    if (e.key === 'ArrowDown' && els.length) {
      e.preventDefault();
      active = (active + 1) % els.length;
      markActive();
      return;
    }
    if (e.key === 'ArrowUp' && els.length) {
      e.preventDefault();
      active = active <= 0 ? els.length - 1 : active - 1;
      markActive();
      return;
    }
    if (e.key === 'Enter') {
      const el = active >= 0 ? els[active] : els[0];
      if (el) { e.preventDefault(); el.click(); }
    }
  });

  document.addEventListener('click', function (e) {
    if (!box.contains(e.target)) clear();
  });
})();
