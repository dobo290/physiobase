// Passkeys (WebAuthn): Anmelden ohne Passwort, Passkey anlegen (eigenes Konto oder Einrichtungs-Link).
(() => {
  "use strict";
  if (!window.PublicKeyCredential) { // Browser ohne Passkeys: Knöpfe ausblenden
    document.querySelectorAll("[data-passkey-login],[data-passkey-register]").forEach((b) => { b.closest(".js-only")?.remove(); b.remove(); });
    return;
  }

  const b64uToBuf = (s) => {
    s = s.replace(/-/g, "+").replace(/_/g, "/"); while (s.length % 4) s += "=";
    return Uint8Array.from(atob(s), (c) => c.charCodeAt(0)).buffer;
  };
  const bufToB64u = (b) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

  const creationOpts = (pk) => PublicKeyCredential.parseCreationOptionsFromJSON ? PublicKeyCredential.parseCreationOptionsFromJSON(pk) : {
    ...pk, challenge: b64uToBuf(pk.challenge), user: { ...pk.user, id: b64uToBuf(pk.user.id) },
    excludeCredentials: (pk.excludeCredentials || []).map((c) => ({ ...c, id: b64uToBuf(c.id) })),
  };
  const requestOpts = (pk) => PublicKeyCredential.parseRequestOptionsFromJSON ? PublicKeyCredential.parseRequestOptionsFromJSON(pk) : {
    ...pk, challenge: b64uToBuf(pk.challenge),
    allowCredentials: (pk.allowCredentials || []).map((c) => ({ ...c, id: b64uToBuf(c.id) })),
  };
  const credJSON = (c) => {
    if (c.toJSON) return c.toJSON();
    const r = c.response, out = { id: c.id, rawId: bufToB64u(c.rawId), type: c.type, response: { clientDataJSON: bufToB64u(r.clientDataJSON) } };
    if (r.attestationObject) { out.response.attestationObject = bufToB64u(r.attestationObject); if (r.getTransports) out.response.transports = r.getTransports(); }
    if (r.authenticatorData) {
      out.response.authenticatorData = bufToB64u(r.authenticatorData); out.response.signature = bufToB64u(r.signature);
      if (r.userHandle) out.response.userHandle = bufToB64u(r.userHandle);
    }
    return out;
  };

  const post = async (url, body) => {
    const res = await fetch(url, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : "{}" });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "Fehler " + res.status);
    return data;
  };
  const show = (el, text, ok) => { if (!el) return; el.hidden = false; el.textContent = text; el.className = ok ? "ok" : "error"; };
  const friendly = (e) => e && e.name === "NotAllowedError" ? "Abgebrochen oder nicht erlaubt." : (e && e.message) || String(e);

  // Anmelden
  document.querySelectorAll("[data-passkey-login]").forEach((btn) => btn.addEventListener("click", async () => {
    const msg = btn.parentElement.querySelector("[data-passkey-msg]");
    btn.disabled = true;
    try {
      const opts = await post("/passkey/login/start");
      const cred = await navigator.credentials.get({ publicKey: requestOpts(opts.publicKey) });
      const r = await post("/passkey/login/fertig", credJSON(cred));
      window.location.href = r.redirect || "/";
    } catch (e) { show(msg, friendly(e), false); btn.disabled = false; }
  }));

  // Passkey anlegen (eigenes Konto oder mit data-token über einen Einrichtungs-Link)
  document.querySelectorAll("[data-passkey-register]").forEach((btn) => btn.addEventListener("click", async () => {
    const box = btn.closest("article, div");
    const msg = box.querySelector("[data-passkey-msg]");
    const name = (box.querySelector("[data-passkey-name]") || {}).value || "";
    const q = new URLSearchParams({ name });
    if (btn.dataset.token) q.set("token", btn.dataset.token);
    btn.disabled = true;
    try {
      const opts = await post("/passkey/registrieren/start?" + q.toString());
      const cred = await navigator.credentials.create({ publicKey: creationOpts(opts.publicKey) });
      await post("/passkey/registrieren/fertig", credJSON(cred));
      show(msg, "Passkey gespeichert.", true);
      setTimeout(() => { window.location.href = btn.dataset.token ? "/login" : "/konto?gespeichert=1#passkeys"; }, 900);
    } catch (e) { show(msg, friendly(e), false); btn.disabled = false; }
  }));
})();
