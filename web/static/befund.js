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
    if (key === "pain") return; // Schmerzquelle hat einen eigenen Handler (Template + Schema)
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

  // ---- Körperschema: Stempel + Freihand-Zeichnen, Undo/Redo ----
  const hiddenStamps = document.getElementById("stamps");
  const hiddenDraw = document.getElementById("drawings");
  if (!hiddenStamps || !hiddenDraw) return;
  const safeParse = (j) => { try { const v = JSON.parse(j); return Array.isArray(v) ? v : []; } catch { return []; } };
  let stamps = safeParse(hiddenStamps.value || "[]");
  let drawings = safeParse(hiddenDraw.value || "[]");
  const NS = "http://www.w3.org/2000/svg";
  const SIZES = [0, 0.055, 0.08, 0.11]; // Symbolgröße relativ zur Höhe (wie befund.StampFrac)
  const LINEW = [0, 0.012, 0.02, 0.03]; // Basis-Strärke relativ zur Höhe (Größenwahl wirkt auch beim Zeichnen)
  const svgs = [...document.querySelectorAll("svg[data-view]")];
  const svgFor = (view) => svgs.find((sv) => +sv.dataset.view === view);

  // Modus Stempel / Zeichnen
  let mode = "stempel";
  const modeBtns = [...document.querySelectorAll(".mode-switch .mode")];
  modeBtns.forEach((b) => b.addEventListener("click", () => {
    mode = b.dataset.mode;
    modeBtns.forEach((x) => x.classList.toggle("active", x === b));
  }));

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

  const appendStamp = (svg, s) => {
    const h = +svg.dataset.h;
    const t = document.createElementNS(NS, "text");
    t.setAttribute("x", (s.x * +svg.dataset.w).toFixed(1));
    t.setAttribute("y", (s.y * +svg.dataset.h).toFixed(1));
    t.setAttribute("font-size", (h * (SIZES[s.z] || SIZES[2])).toFixed(1));
    t.setAttribute("fill", s.c);
    t.setAttribute("text-anchor", "middle");
    t.setAttribute("dominant-baseline", "central");
    t.textContent = s.s === "pain" ? "⚡P" + (s.n || 1) : (chars[s.s] || "?");
    svg.appendChild(t);
  };
  const appendPath = (svg, dr) => {
    const w = +svg.dataset.w, h = +svg.dataset.h;
    let d = "";
    dr.pts.forEach((p, i) => { d += (i ? "L " : "M ") + (p.x * w).toFixed(1) + " " + (p.y * h).toFixed(1) + " "; });
    const path = document.createElementNS(NS, "path");
    path.setAttribute("d", d);
    path.setAttribute("stroke", dr.c);
    path.setAttribute("stroke-width", Math.max(0.5, (dr.w || 0.02) * h));
    path.setAttribute("fill", "none");
    path.setAttribute("stroke-linecap", "round");
    path.setAttribute("stroke-linejoin", "round");
    svg.appendChild(path);
  };
  const redraw = (svg, view) => {
    if (!svg) return;
    while (svg.lastElementChild) svg.removeChild(svg.lastElementChild);
    drawings.filter((dr) => dr.v === view).forEach((dr) => appendPath(svg, dr));
    stamps.filter((st) => st.v === view).forEach((st) => appendStamp(svg, st));
  };
  const save = () => { hiddenStamps.value = JSON.stringify(stamps); hiddenDraw.value = JSON.stringify(drawings); dirty = true; };

  // Undo/Redo (Stempel und Striche gemeinsam)
  const undoStack = [], redoStack = [];
  const undoBtn = document.getElementById("btn-undo");
  const redoBtn = document.getElementById("btn-redo");
  const updateArrows = () => { undoBtn.disabled = undoStack.length === 0; redoBtn.disabled = redoStack.length === 0; };
  const clamp01 = (v) => Math.min(1, Math.max(0, v));
  const ptAt = (svg, ev) => { const r = svg.getBoundingClientRect(); return { x: +clamp01((ev.clientX - r.left) / r.width).toFixed(4), y: +clamp01((ev.clientY - r.top) / r.height).toFixed(4) }; };

  // ---- Schmerzquellen: Körperschema (⚡P1…) und Tabelle bleiben synchron ----
  const painTable = () => document.querySelector('table[data-table="pain"]');
  const painCount = () => { const n = document.querySelector('input[name="t.pain.n"]'); return n ? parseInt(n.value || "0", 10) : 0; };
  const painStampCount = () => stamps.filter((st) => st.s === "pain").length;
  const setPainCount = (k) => { const n = document.querySelector('input[name="t.pain.n"]'); if (n) n.value = k; };
  const addPainRow = () => {
    const tb = painTable()?.querySelector("tbody");
    const tpl = document.getElementById("pain-row-template");
    if (!tb || !tpl) return;
    const tr = tpl.content.firstElementChild.cloneNode(true);
    tr.querySelectorAll("input,select").forEach((el) => {
      el.name = el.name.replace(/__i__/g, painCount());
      if (el.tagName === "SELECT") el.selectedIndex = 0; else el.value = "";
      if (el.dataset && el.dataset.dependsOnCol) { const cell = el.closest("td"); if (cell) cell.hidden = true; }
    });
    tb.appendChild(tr);
    setPainCount(painCount() + 1);
  };
  const renumberPain = () => {
    const pstamps = stamps.filter((st) => st.s === "pain");
    pstamps.forEach((st, i) => { st.n = i + 1; });
    const tb = painTable()?.querySelector("tbody");
    if (tb) [...tb.querySelectorAll("tr")].forEach((tr, i) => {
      tr.dataset.painRow = "P" + (i + 1);
      const nr = tr.querySelector(".pain-nr"); if (nr) nr.textContent = "P" + (i + 1);
    });
    setPainCount(pstamps.length);
  };
  const addPainStamp = (svg, e, num) => {
    const p = ptAt(svg, e);
    const st = { v: +svg.dataset.view, x: p.x, y: p.y, s: "pain", c: color?.value || "#d32f2f", z: +(size?.value || 2), n: num || painStampCount() + 1 };
    stamps.push(st); appendStamp(svg, st);
    addPainRow(); renumberPain();
    undoStack.push({ view: +svg.dataset.view, type: "stamp", st }); redoStack.length = 0;
    save(); updateArrows();
  };
  const addPainStampFromRow = () => { // "+ Zeile" in der Tabelle → nur Punkt auf dem Schema (Zeile hat addrow schon erzeugt)
    const svg = svgFor(0);
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const p = { x: +0.5.toFixed(4), y: +0.55.toFixed(4) }; // Standardposition, verschiebbar per Undo/Neuplatzieren
    const st = { v: 0, x: p.x, y: p.y, s: "pain", c: color?.value || "#d32f2f", z: +(size?.value || 2), n: painCount() };
    stamps.push(st); appendStamp(svg, st);
    renumberPain(); save(); updateArrows();
    undoStack.push({ view: 0, type: "stamp", st }); redoStack.length = 0;
  };
  const removePainRow = (tr) => {
    const idx = [...tr.parentElement.children].indexOf(tr);
    const num = idx + 1;
    tr.remove();
    const i = stamps.findIndex((st) => st.s === "pain" && st.n === num);
    if (i >= 0) { stamps.splice(i, 1); }
    renumberPain(); save();
    undoStack.length = 0; redoStack.length = 0; updateArrows();
  };

  svgs.forEach((svg) => {
    const view = +svg.dataset.view;
    const w = +svg.dataset.w, h = +svg.dataset.h;
    redraw(svg, view);

    svg.addEventListener("pointerdown", (e) => {
      e.preventDefault();
      if (mode === "stempel") {
        const s = { v: view, ...ptAt(svg, e), s: sym, c: color?.value || "#d32f2f", z: +(size?.value || 2) };
        stamps.push(s); appendStamp(svg, s);
        undoStack.push({ view, type: "stamp", st: s }); redoStack.length = 0;
        save(); updateArrows();
        return;
      }
      if (mode === "schmerz") { addPainStamp(svg, e); return; }
      // Zeichnen: live ein Pfad-Element, das bei jeder Bewegung wächst
      svg.setPointerCapture(e.pointerId);
      const z = +(size?.value || 2);
      const stroke = { v: view, c: color?.value || "#000000", w: LINEW[z], pts: [ptAt(svg, e)] };
      const pathEl = document.createElementNS(NS, "path");
      pathEl.setAttribute("stroke", stroke.c);
      pathEl.setAttribute("fill", "none");
      pathEl.setAttribute("stroke-linecap", "round");
      pathEl.setAttribute("stroke-linejoin", "round");
      svg.appendChild(pathEl);
      const updatePath = () => {
        let d = "";
        stroke.pts.forEach((p, i) => { d += (i ? "L " : "M ") + (p.x * w).toFixed(1) + " " + (p.y * h).toFixed(1) + " "; });
        pathEl.setAttribute("d", d);
        pathEl.setAttribute("stroke-width", Math.max(0.5, (stroke.w * (0.5 + (stroke.pressure || 0.5))) * h).toFixed(1));
      };
      updatePath();
      const move = (ev) => {
        ev.preventDefault();
        stroke.pressure = ev.pressure || 0.5;
        const p = ptAt(svg, ev);
        const last = stroke.pts[stroke.pts.length - 1];
        if (last && Math.abs(p.x - last.x) < 0.003 && Math.abs(p.y - last.y) < 0.003) return; // Rauschen filtern
        stroke.pts.push(p);
        updatePath();
      };
      const up = (ev) => {
        svg.removeEventListener("pointermove", move);
        svg.removeEventListener("pointerup", up);
        svg.removeEventListener("pointercancel", up);
        if (stroke.pts.length < 2) { pathEl.remove(); return; }
        drawings.push(stroke);
        undoStack.push({ view, type: "draw", st: stroke }); redoStack.length = 0;
        save(); updateArrows();
      };
      svg.addEventListener("pointermove", move);
      svg.addEventListener("pointerup", up);
      svg.addEventListener("pointercancel", up);
    });
  });

  undoBtn.addEventListener("click", () => {
    const a = undoStack.pop(); if (!a) return;
    if (a.type === "stamp") {
      stamps = stamps.filter((x) => x !== a.st);
      if (a.st.s === "pain") { // passende Zeile entfernen
        const tb = painTable()?.querySelector("tbody");
        const tr = tb?.querySelector('tr[data-pain-row="P' + a.st.n + '"]');
        if (tr) tr.remove();
      }
    } else drawings = drawings.filter((x) => x !== a.st);
    renumberPain();
    redraw(svgFor(a.view), a.view);
    redoStack.push(a); save(); updateArrows();
  });
  redoBtn.addEventListener("click", () => {
    const a = redoStack.pop(); if (!a) return;
    if (a.type === "stamp") {
      stamps.push(a.st);
      if (a.st.s === "pain") addPainRow();
    } else drawings.push(a.st);
    renumberPain();
    redraw(svgFor(a.view), a.view);
    undoStack.push(a); save(); updateArrows();
  });

  // "+ Zeile" in der Schmerz-Tabelle → Zeile aus Template + Punkt auf dem Schema
  document.querySelectorAll("button.addrow[data-table='pain']").forEach((btn) => {
    btn.addEventListener("click", () => { addPainRow(); addPainStampFromRow(); });
  });
  // "✕" in einer Schmerz-Zeile → Zeile + Punkt entfernen
  document.querySelectorAll("table[data-table='pain']").forEach((tb) => {
    tb.addEventListener("click", (e) => {
      const btn = e.target.closest("[data-pain-rm]");
      if (!btn) return;
      removePainRow(btn.closest("tr"));
    });
  });
  // Subtyp-Spalte: nur sichtbar, wenn Schmerzmechanismus = nozizeptiv
  document.addEventListener("change", (e) => {
    const sel = e.target;
    if (!(sel instanceof HTMLSelectElement) || !sel.dataset.dependsOnCol) return;
    const cell = sel.closest("td");
    if (cell && cell.classList.contains("subcell")) {
      cell.hidden = sel.value !== sel.dataset.dependsValue;
    }
  });
  updateArrows();
})();
