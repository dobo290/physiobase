// Suchfeld für Tests im Befund: Vorschläge laden und Test verknüpfen.
(function () {
  const box = document.querySelector('.q-add');
  const input = box.querySelector('input[type="search"]');
  const list = box.querySelector('.suggest');
  if (!box || !input || !list) return;

  const action = box.dataset.action || ('/befund/' + box.dataset.befund + '/test');
  let timer = null;

  function clear() {
    list.hidden = true;
    list.innerHTML = '';
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
    form.submit();
    input.value = title;
  }

  async function search(q) {
    const res = await fetch('/api/tests?q=' + encodeURIComponent(q), {
      headers: { Accept: 'application/json' },
    });
    if (!res.ok) return [];
    return res.json();
  }

  input.addEventListener('input', function () {
    const q = input.value.trim();
    clearTimeout(timer);
    if (q.length < 1) { clear(); return; }
    timer = setTimeout(async function () {
      let items = [];
      try { items = await search(q); } catch (e) { return; }
      if (!items.length) {
        list.innerHTML = '<li class="muted">kein Treffer</li>';
        list.hidden = false;
        return;
      }
      list.innerHTML = '';
      items.forEach(function (it) {
        const li = document.createElement('li');
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'suggest-item';
        btn.textContent = it.title;
        btn.addEventListener('click', function () { pick(it.id, it.title); });
        li.appendChild(btn);
        list.appendChild(li);
      });
      list.hidden = false;
    }, 150);
  });

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') clear();
    if (e.key === 'Enter') {
      const first = list.querySelector('.suggest-item');
      if (first) { e.preventDefault(); first.click(); }
    }
  });

  document.addEventListener('click', function (e) {
    if (!box.contains(e.target)) clear();
  });
})();
