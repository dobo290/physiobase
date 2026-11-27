// Sidebar: ein-/ausklappen (Desktop) bzw. als Schublade öffnen (schmal).
// Ohne JavaScript funktioniert der Knopf als normales Formular (POST /sidebar/umschalten).
(() => {
  "use strict";
  document.documentElement.classList.add("js");

  const mobile = () => window.matchMedia("(max-width: 900px)").matches;

  document.addEventListener("click", (evt) => {
    const btn = evt.target.closest("[data-sidebar-toggle]");
    if (!btn) return;
    evt.preventDefault();
    if (mobile()) {
      document.body.classList.toggle("sidebar-open");
      btn.setAttribute("aria-expanded", document.body.classList.contains("sidebar-open") ? "true" : "false");
      return;
    }
    document.body.classList.toggle("sidebar-collapsed");
    const collapsed = document.body.classList.contains("sidebar-collapsed");
    document.cookie = "physiobase_sidebar=" + (collapsed ? "1" : "0") + "; path=/; max-age=31536000; samesite=lax";
    btn.setAttribute("aria-expanded", collapsed ? "false" : "true");
  });

  // Schublade schließen: Escape, Klick auf den Inhalt, Navigation
  document.addEventListener("keydown", (evt) => {
    if (evt.key === "Escape") document.body.classList.remove("sidebar-open");
  });
  const main = document.querySelector(".main");
  if (main) {
    main.addEventListener("click", () => document.body.classList.remove("sidebar-open"));
  }
})();
