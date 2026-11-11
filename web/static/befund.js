// Befund: Körperschema stempeln, Tabellenzeilen ergänzen, Warnung bei ungespeicherten Änderungen.
// Ohne JavaScript funktioniert alles außer dem Stempeln (Formular, Tabellen per Speichern).
(() => {
  "use strict";
  document.documentElement.classList.add("js");

  // ---- Fragebogen-Editor im Eintragsformular: Blöcke an-/ausschalten ----
  document.querySelectorAll('input[name="fragebogen_an"]').forEach((cb) => {
    const box = document.getElementById("_fq");
    if (!box) return;
    const show = () => { box.hidden = !cb.checked; };
    cb.addEventListener("change", show);
    show();
  });

  // ---- Tabellen: weitere Zeile (alle Seiten) ----
  // Normal: neue leere Zeile. Mit data-step-days (Terminzeilen): letzte Zeile übernehmen,
  // Datum um so viele Tage weiter – so entsteht eine Terminserie ohne Neuladen der Seite.
  document.querySelectorAll("button.addrow").forEach((btn) => btn.addEventListener("click", () => {
    const key = btn.dataset.table;
    const scope = btn.closest("form") || document;
    const table = scope.querySelector(`table[data-table="${key}"]`);
    const tbody = table?.querySelector("tbody");
    const count = scope.querySelector(`input[name="t.${key}.n"], input[name="d.n"]`);
    if (!tbody || !count || !tbody.lastElementChild) return;
    const step = parseInt(table.dataset.stepDays || "0", 10);
    const n = parseInt(count.value, 10) || 0;
    const last = tbody.lastElementChild;
    const tr = last.cloneNode(true);
    const src = [...last.querySelectorAll("input,select")];
    tr.querySelectorAll("input,select").forEach((el, i) => {
      el.name = el.name.replace(/^(?:t\.[^.]+\." + "|d\.)\d+(\.)/, `$1${n}$2`);
      if (!step) { if (el.tagName === "SELECT") el.selectedIndex = 0; else el.value = ""; return; }
      if (el.tagName === "SELECT") { el.selectedIndex = src[i].selectedIndex; return; }
      el.value = src[i].value;
      if (el.type === "date" && el.value) {
        const d = new Date(el.value + "T12:00:00");
        d.setDate(d.getDate() + step);
        el.value = d.toISOString().slice(0, 10);
      }
    });
    tbody.appendChild(tr);
    count.value = n + 1;
    tr.querySelector("input,select")?.focus();
  }));

  const form = document.getElementById("befund-form");
  if (!form) return;

  let dirty = false;
  form.addEventListener("input", () => { dirty = true; });
  form.addEventListener("submit", () => { dirty = false; });
  window.addEventListener("beforeunload", (e) => { if (dirty) { e.preventDefault(); e.returnValue = ""; } });
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") { e.preventDefault(); dirty = false; form.requestSubmit(); }
  });

  // ---- Körperschema ----
  const hidden = document.getElementById("stamps");
  if (!hidden) return;
  let stamps = [];
  try { stamps = JSON.parse(hidden.value || "[]") || []; } catch { stamps = []; }
  const symButtons = [...document.querySelectorAll(".body-tools .sym")];
  const chars = Object.fromEntries(symButtons.map((b) => [b.dataset.sym, b.textContent]));
  let sym = symButtons.find((b) => b.classList.contains("active"))?.dataset.sym || "pain";
  symButtons.forEach((b) => b.addEventListener("click", () => {
    symButtons.forEach((x) => x.classList.remove("active"));
    b.classList.add("active");
    sym = b.dataset.sym;
  }));
  const color = document.getElementById("stamp-color");
  const size = document.getElementById("stamp-size");
  const SIZES = [0, 7, 11, 16]; // gleich wie befund.StampSize
  const NS = "http://www.w3.org/2000/svg";

  const draw = (svg, s) => {
    const t = document.createElementNS(NS, "text");
    t.setAttribute("x", (s.x * +svg.dataset.w).toFixed(1));
    t.setAttribute("y", (s.y * +svg.dataset.h).toFixed(1));
    t.setAttribute("font-size", SIZES[s.z] || 11);
    t.setAttribute("fill", s.c);
    t.setAttribute("text-anchor", "middle");
    t.setAttribute("dominant-baseline", "central");
    t.textContent = chars[s.s] || "?";
    svg.appendChild(t);
  };
  const save = () => { hidden.value = JSON.stringify(stamps); dirty = true; };

  document.querySelectorAll("svg[data-view]").forEach((svg) => svg.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    const r = svg.getBoundingClientRect();
    const s = {
      v: +svg.dataset.view,
      x: +Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)).toFixed(4),
      y: +Math.min(1, Math.max(0, (e.clientY - r.top) / r.height)).toFixed(4),
      s: sym, c: color?.value || "#d32f2f", z: +(size?.value || 2),
    };
    stamps.push(s);
    draw(svg, s);
    save();
  }));
  document.getElementById("stamp-undo")?.addEventListener("click", () => {
    const s = stamps.pop();
    if (!s) return;
    document.querySelector(`svg[data-view="${s.v}"]`)?.lastElementChild?.remove();
    save();
  });
})();
