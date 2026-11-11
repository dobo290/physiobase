// Demo-Banner: Einwilligung zur Statistik. Ohne Zustimmung wird nichts gezählt.
(() => {
  "use strict";
  const banner = document.getElementById("demo-consent");
  if (!banner) return;
  const hasConsent = document.cookie.split("; ").some((c) => c.startsWith("physiobase_demo_consent="));
  if (!hasConsent) banner.hidden = false;
  const setConsent = () => {
    document.cookie = "physiobase_demo_consent=1; path=/; max-age=31536000; SameSite=Lax";
    banner.hidden = true;
    location.reload(); // Statistik zählt jetzt
  };
  banner.querySelector("[data-demo-consent-accept]").addEventListener("click", setConsent);
  // "Nein, zurück zu meiner Seite": zurück zur Seite, von der man kam (der Browser
  // gibt die eigene Startseite nicht preis; das ist die übliche datenschutzkonforme Lösung).
  banner.querySelector("[data-demo-consent-leave]").addEventListener("click", () => {
    banner.hidden = true;
    if (history.length > 1) history.back();
    else if (window.opener) window.close();
    else window.location.replace("/");
  });
})();
