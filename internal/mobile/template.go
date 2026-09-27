package mobile

// pageSource is the whole application markup. Everything the user sees is
// rendered from here, so there is no HTML asset to keep in sync with the app.
//
// The two pane wrappers exist for the wide layout: on a phone they are plain
// stacked blocks, and past the two-pane breakpoint they become grid areas.
const pageSource = `
<div class="shell" dir="{{if .RTL}}rtl{{else}}ltr{{end}}" data-theme="{{.Theme}}">

  <header class="brand">
    {{.Logo}}
    <div class="brand-text">
      <h1>PANTEGNOS</h1>
      <p>{{t "brand_tagline"}}</p>
    </div>
    <button class="icon" data-act="theme" aria-label="{{t "cd_theme_toggle"}}">
      {{if .Dark}}<svg viewBox="0 0 24 24"><path d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10Zm0-5v2m0 18v2M4.2 4.2l1.4 1.4m12.8 12.8 1.4 1.4M2 12h2m18 0h-2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" fill="none"/></svg>{{else}}<svg viewBox="0 0 24 24"><path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5Z" fill="currentColor"/></svg>{{end}}
    </button>
    <button class="icon" data-act="settings" aria-label="{{t "cd_about"}}">
      <svg viewBox="0 0 24 24"><path d="M4 7h10M18 7h2M4 17h4M12 17h8" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/><circle cx="16" cy="7" r="2.4" stroke="currentColor" stroke-width="1.8" fill="none"/><circle cx="10" cy="17" r="2.4" stroke="currentColor" stroke-width="1.8" fill="none"/></svg>
    </button>
  </header>

  <div class="pane-intake">
    <div class="pill">
      <svg class="shield" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2 4 5.5V11c0 5 3.4 9.2 8 11 4.6-1.8 8-6 8-11V5.5L12 2Z" fill="var(--grad-a)"/></svg>
      <span>{{t "privacy_chip"}}</span>
      <span class="version">{{t "version_chip" .Version}}</span>
    </div>

    <div class="strip">
      <span class="dot ok"></span>
      <strong>{{t "engine_ready"}}</strong>
    </div>

    <div class="drop" data-act="pick" role="button" tabindex="0"
         aria-label="{{t "drop_title"}}. {{t "drop_hint"}}">
      <div class="drop-icon">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h9A1.5 1.5 0 0 1 21 10v8a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18V7.5Z" stroke="currentColor" stroke-width="1.7" fill="none" stroke-linejoin="round"/></svg>
      </div>
      <strong>{{t "drop_title"}}</strong>
      <span class="hint">{{t "drop_hint"}}</span>
      <div class="drop-actions">
        <button class="btn primary" data-act="pick">{{t "action_add"}}</button>
        <button class="btn ghost" data-act="paste">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 4h6v2H9zM8 5.5H6.5A1.5 1.5 0 0 0 5 7v12.5A1.5 1.5 0 0 0 6.5 21h11a1.5 1.5 0 0 0 1.5-1.5V7a1.5 1.5 0 0 0-1.5-1.5H16" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linejoin="round"/></svg>
          {{t "action_paste"}}
        </button>
      </div>
    </div>

    <p class="label">{{t "supported_formats"}}</p>
    <div class="chips">
      {{range .Formats}}<span class="chip">{{.Ext}}</span>{{end}}
    </div>
  </div>

  <div class="pane-queue">
  {{if .Jobs}}
    <div class="queue-head">
      <strong>{{t "section_queue"}}</strong>
      <button class="btn ghost sm" data-act="clear">{{t "action_clear"}}</button>
    </div>
  {{end}}
  {{range .Jobs}}
  <article class="card tone-{{.Tone}}" data-id="{{.ID}}">
    <div class="card-head">
      <span class="ext">{{if .Ext}}{{.Ext}}{{else}}&#128274;{{end}}</span>
      <div class="card-title">
        <strong>{{.Name}}</strong>
        {{if .Subtitle}}<span class="sub">{{.Subtitle}}</span>{{end}}
      </div>
      <span class="status {{.Tone}}">
        <span class="dot {{.Tone}}{{if .Busy}} pulse{{end}}"></span>{{.Status}}
      </span>
    </div>

    {{if .Err}}
    <div class="alert">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 1.5 21h21L12 3Zm0 6v6m0 3v.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" fill="none"/></svg>
      <span>{{.Err}}</span>
    </div>
    {{end}}

    <div class="card-actions">
      {{if .CanRetry}}
      <button class="btn ghost sm" data-act="retry" data-id="{{.ID}}">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 12a8 8 0 1 1-2.6-5.9M20 4v4h-4" stroke="currentColor" stroke-width="1.8" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
        {{t .RetryKey}}
      </button>
      {{end}}
      <span class="spacer"></span>
      {{if .Expanded}}
      <button class="icon sm" data-act="expand" data-id="{{.ID}}" aria-label="{{t "action_collapse"}}">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 15 6-6 6 6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      {{end}}
      <button class="icon sm" data-act="remove" data-id="{{.ID}}" aria-label="{{t "action_remove"}}">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
      </button>
    </div>

    {{if .Expanded}}
    <div class="result">
      {{if .Empty}}<p class="muted">{{t "viewer_empty"}}</p>
      {{else}}<pre class="mono{{if not .Wrap}} nowrap{{end}}">{{.Text}}</pre>{{end}}
      {{if .Truncate}}<p class="muted tiny">{{t "viewer_truncated"}}</p>{{end}}
      <div class="tools">
        <button class="btn ghost sm" data-act="copy" data-id="{{.ID}}">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 9h10v10a2 2 0 0 1-2 2h-8a2 2 0 0 1-2-2V9Z M15 5v2H7a2 2 0 0 0-2 2v8" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linejoin="round"/></svg>
          {{t "action_copy"}}
        </button>
        <button class="btn ghost sm" data-act="share" data-id="{{.ID}}">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0-12L8 7m4-4 4 4M5 14v5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-5" stroke="currentColor" stroke-width="1.7" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
          {{t "action_share"}}
        </button>
        <button class="btn ghost sm" data-act="save" data-id="{{.ID}}">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 4h11l3 3v13H5V4Zm3 0v6h7V4M8 20v-6h8v6" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linejoin="round"/></svg>
          {{t "action_save"}}
        </button>
        <button class="btn ghost sm" data-act="wrap" data-id="{{.ID}}">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M4 12h13a3 3 0 1 1 0 6h-3m0 0 2-2m-2 2 2 2M4 18h3" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
          {{if .Wrap}}{{t "viewer_wrap"}}{{else}}{{t "viewer_scroll"}}{{end}}
        </button>
      </div>
    </div>
    {{end}}
  </article>
  {{end}}
  {{if not .Jobs}}
  <div class="empty">
    <strong>{{t "cd_empty_title"}}</strong>
    <span>{{t "cd_empty_body"}}</span>
  </div>
  {{end}}
  </div>

  <footer class="page-foot">
    <p class="note">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 11V8a5 5 0 0 1 10 0v3" stroke="currentColor" stroke-width="1.7" fill="none" stroke-linecap="round"/><rect x="5" y="11" width="14" height="9" rx="2" stroke="currentColor" stroke-width="1.7" fill="none"/></svg>
      <span>{{t "about_privacy_body"}}</span>
    </p>
    <p class="tiny">{{t "about_legal_body"}}</p>
  </footer>
</div>

{{if .Password}}
<div class="scrim" data-act="cancel-password"></div>
<div class="sheet" role="dialog" aria-modal="true" aria-label="{{t "password_title"}}">
  <div class="grip"></div>
  <div class="sheet-head">
    <span class="mini-emblem">{{.Logo}}</span>
    <div>
      <strong>{{t "password_title"}}</strong>
      <span class="accent">{{.Password.Name}}</span>
    </div>
  </div>
  <p class="muted">{{.Password.Body}}</p>
  <label class="field{{if .Password.Error}} has-error{{end}}">
    <span>{{t "password_label"}}</span>
    <input id="pw" type="password" autocomplete="off" inputmode="text"
           data-error="{{if .Password.Error}}true{{else}}false{{end}}"
           aria-label="{{t "password_label"}}">
    <button class="icon in-field" data-act="toggle-pw" aria-label="{{t "password_show"}}">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.8-6 10-6 10 6 10 6-3.8 6-10 6-10-6-10-6Z" stroke="currentColor" stroke-width="1.6" fill="none"/><circle cx="12" cy="12" r="2.6" stroke="currentColor" stroke-width="1.6" fill="none"/></svg>
    </button>
  </label>
  {{if .Password.Error}}
  <p class="field-error">
    {{.Password.Error}}
    <span class="tiny">{{.Password.Raw}}</span>
  </p>
  {{end}}
  <div class="sheet-actions">
    <button class="btn ghost" data-act="cancel-password">{{t "password_cancel"}}</button>
    <button class="btn primary" data-act="submit-password" id="pw-submit">{{t "password_confirm"}}</button>
  </div>
</div>
{{end}}

{{if .ShowSettings}}
<div class="scrim" data-act="close-settings"></div>
<div class="sheet settings" role="dialog" aria-modal="true" aria-label="{{t "about_title"}}">
  <div class="grip"></div>
  <div class="sheet-head">
    <span class="mini-emblem">{{.Logo}}</span>
    <div>
      <strong>{{t "about_title"}}</strong>
      <span class="accent">{{t "about_version" .Version}}</span>
    </div>
  </div>

  <h3>{{t "settings_theme"}}</h3>
  <div class="seg">
    <button data-act="theme-set" data-value="system" aria-pressed="{{if eq .Theme "system"}}true{{else}}false{{end}}">{{t "theme_system"}}</button>
    <button data-act="theme-set" data-value="light" aria-pressed="{{if eq .Theme "light"}}true{{else}}false{{end}}">{{t "theme_light"}}</button>
    <button data-act="theme-set" data-value="dark" aria-pressed="{{if eq .Theme "dark"}}true{{else}}false{{end}}">{{t "theme_dark"}}</button>
  </div>

  <h3>{{t "about_privacy_title"}}</h3>
  <p class="note">
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 11V8a5 5 0 0 1 10 0v3" stroke="currentColor" stroke-width="1.7" fill="none" stroke-linecap="round"/><rect x="5" y="11" width="14" height="9" rx="2" stroke="currentColor" stroke-width="1.7" fill="none"/></svg>
    <span>{{t "about_privacy_body"}}</span>
  </p>

  <h3>{{t "about_formats_title"}}</h3>
  <ul>
    {{range .Formats}}<li><span class="chip">{{.Ext}}</span><span>{{.Desc}}</span></li>{{end}}
  </ul>

  <h3>{{t "about_legal_title"}}</h3>
  <p class="tiny">{{t "about_legal_body"}}</p>
  <p class="tiny">{{t "about_credits"}}</p>

  <div class="sheet-actions">
    <button class="btn ghost" data-act="github">{{t "about_github"}}</button>
    <button class="btn ghost" data-act="website">{{t "about_website"}}</button>
    <span class="spacer"></span>
    <button class="btn primary" data-act="close-settings">{{t "action_dismiss"}}</button>
  </div>
</div>
{{end}}

<div class="toast" id="toast" hidden></div>
`
