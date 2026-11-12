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
    t.textContent = chars[s.s] || "?";
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
    if (a.type === "stamp") stamps = stamps.filter((x) => x !== a.st);
    else drawings = drawings.filter((x) => x !== a.st);
    redraw(svgFor(a.view), a.view);
    redoStack.push(a); save(); updateArrows();
  });
  redoBtn.addEventListener("click", () => {
    const a = redoStack.pop(); if (!a) return;
    if (a.type === "stamp") stamps.push(a.st); else drawings.push(a.st);
    redraw(svgFor(a.view), a.view);
    undoStack.push(a); save(); updateArrows();
  });
  updateArrows();
})();
