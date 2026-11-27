// Kalender: Ziehen (verschieben) und Ziehen an der Unterkante (Dauer ändern).
// Freiwillige Erleichterung – ohne JavaScript geht alles über den Editor daneben.
// Gespeichert wird über denselben Server-Endpunkt wie das Formular.
(() => {
  "use strict";
  const svg = document.querySelector(".cal-svg");
  if (!svg) return;

  const grid = parseInt(svg.dataset.calGrid, 10) || 10;
  const startMin = parseInt(svg.dataset.calStartmin, 10) || 480;
  const px = parseFloat(svg.dataset.calPxmin) || 1;
  const header = parseFloat(svg.dataset.calHeader) || 42;
  const gutter = parseFloat(svg.dataset.calGutter) || 62;
  const day = svg.dataset.calDay || "";
  const cols = (svg.dataset.calTherapists || "").split(",");
  const vb = svg.viewBox.baseVal;
  const colW = cols.length ? (vb.width - gutter - 4) / cols.length : vb.width;

  const hhmm = (mins) => String(Math.floor(mins / 60)).padStart(2, "0") + ":" + String(mins % 60).padStart(2, "0");
  const point = (evt) => {
    const p = svg.createSVGPoint();
    p.x = evt.clientX;
    p.y = evt.clientY;
    const m = svg.getScreenCTM();
    return m ? p.matrixTransform(m.inverse()) : p;
  };
  const snap = (value) => Math.round(value / grid) * grid;

  let drag = null;
  let suppressClick = false;

  svg.addEventListener("pointerdown", (evt) => {
    if (evt.button !== 0) return;
    const link = evt.target.closest("a.cal-link");
    if (!link) return;
    const rect = link.querySelector("rect");
    if (!rect) return;
    const p = point(evt);
    const y = parseFloat(rect.getAttribute("y"));
    const h = parseFloat(rect.getAttribute("height"));
    const x = parseFloat(rect.getAttribute("x"));
    const w = parseFloat(rect.getAttribute("width"));
    const ghost = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    ghost.setAttribute("class", "cal-drag");
    ghost.setAttribute("x", x);
    ghost.setAttribute("y", y);
    ghost.setAttribute("width", w);
    ghost.setAttribute("height", h);
    ghost.setAttribute("rx", 4);
    svg.appendChild(ghost);
    drag = {
      link, rect, ghost, y, h, x, w,
      resize: p.y > y + h - 8,
      startY: p.y, startX: p.x,
      startMin: parseInt(link.dataset.startmin, 10) || startMin,
      duration: parseInt(link.dataset.duration, 10) || grid,
      col: parseInt(link.dataset.col, 10) || 0,
      newCol: null,
      moved: false,
    };
    svg.setPointerCapture(evt.pointerId);
    evt.preventDefault();
  });

  svg.addEventListener("pointermove", (evt) => {
    if (!drag) return;
    const p = point(evt);
    const dy = p.y - drag.startY;
    const dx = p.x - drag.startX;
    if (Math.abs(dy) + Math.abs(dx) > 3) drag.moved = true;
    if (drag.resize) {
      const nh = Math.max(grid, snap((drag.h + dy) / px)) * px;
      drag.ghost.setAttribute("height", Math.max(grid * px, nh));
      return;
    }
    const ny = drag.y + snap(dy / px) * px;
    const maxY = vb.height - drag.h - 12;
    const clamped = Math.max(header, Math.min(maxY, ny));
    drag.ghost.setAttribute("y", clamped);
    const col = Math.max(0, Math.min(cols.length - 1, Math.floor((p.x - gutter) / colW)));
    drag.newCol = col;
    drag.ghost.setAttribute("x", gutter + col * colW + 2);
    drag.ghost.setAttribute("width", colW - 4);
  });

  const finish = async (evt) => {
    if (!drag) return;
    const d = drag;
    drag = null;
    const ghostY = parseFloat(d.ghost.getAttribute("y"));
    const ghostH = parseFloat(d.ghost.getAttribute("height"));
    d.ghost.remove();
    if (svg.hasPointerCapture?.(evt.pointerId)) svg.releasePointerCapture(evt.pointerId);
    if (!d.moved) return; // Klick: Editor öffnen (normale Navigation)
    suppressClick = true;
    setTimeout(() => { suppressClick = false; }, 0);

    let newStart = d.startMin;
    let newDuration = d.duration;
    let newCol = d.col;
    if (d.resize) {
      newDuration = Math.max(grid, snap(ghostH / px));
    } else {
      newStart = startMin + snap((ghostY - header) / px);
      newCol = d.newCol === null ? d.col : d.newCol;
    }
    if (newStart === d.startMin && newDuration === d.duration && newCol === d.col) return;

    const body = new URLSearchParams();
    body.set("date", day);
    body.set("time", hhmm(newStart));
    body.set("duration", String(newDuration));
    body.set("therapist", cols[newCol] || "0");

    const url = "/termin/" + d.link.dataset.appt + "/verschieben";
    const post = (extra) => fetch(url, {
      method: "POST",
      headers: { Accept: "application/json" },
      body: extra ? body.toString() + "&" + extra : body.toString(),
    });

    try {
      let res = await post("");
      if (res.status === 409) {
        const data = await res.json().catch(() => null);
        const list = data && data.conflicts ? data.conflicts.join("\n") : "Überschneidung";
        if (!confirm("Dieser Termin überschneidet sich:\n\n" + list + "\n\nTrotzdem speichern?")) {
          window.location.reload();
          return;
        }
        res = await post("bestaetigt=1");
      }
      if (res.ok) {
        window.location.reload();
      } else {
        alert("Der Termin konnte nicht gespeichert werden.");
        window.location.reload();
      }
    } catch {
      alert("Der Termin konnte nicht gespeichert werden.");
      window.location.reload();
    }
  };

  svg.addEventListener("pointerup", finish);
  svg.addEventListener("pointercancel", finish);

  svg.addEventListener("click", (evt) => {
    if (!suppressClick) return;
    evt.preventDefault();
    evt.stopPropagation();
  }, true);
})();
