package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/language"
)

// lang is a language the public pages come in. The admin pages are English
// only.
type lang string

const (
	english lang = "en"
	swedish lang = "sv"
)

// langs are the supported languages. The first is the default, for browsers
// that ask for neither.
var langs = []lang{english, swedish}

var langMatcher = language.NewMatcher([]language.Tag{language.English, language.Swedish})

const (
	// langCookie holds the language picked with the switch in the footer. It
	// wins over the browser's Accept-Language.
	langCookie    = "lang"
	langCookieAge = 365 * 24 * time.Hour
)

// requestLang is the language to answer r in: the one picked in the footer,
// else the best match for the browser's languages, else English.
func requestLang(r *http.Request) lang {
	if c, err := r.Cookie(langCookie); err == nil {
		if l, ok := parseLang(c.Value); ok {
			return l
		}
	}
	_, i := language.MatchStrings(langMatcher, r.Header.Get("Accept-Language"))
	return langs[i]
}

func parseLang(s string) (lang, bool) {
	for _, l := range langs {
		if string(l) == s {
			return l, true
		}
	}
	return "", false
}

// setLang is the language switch in the footer: GET /lang?l=sv remembers the
// language in a cookie and goes back to the page it was clicked on.
func (s *server) setLang(w http.ResponseWriter, r *http.Request) {
	if l, ok := parseLang(r.URL.Query().Get("l")); ok {
		http.SetCookie(w, &http.Cookie{
			Name:     langCookie,
			Value:    string(l),
			Path:     "/",
			MaxAge:   int(langCookieAge / time.Second),
			HttpOnly: true,
			Secure:   s.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		})
	}
	http.Redirect(w, r, backTo(r), http.StatusSeeOther)
}

// backTo is the page on this site that r came from, or the front page. The
// Referer keeps the query, so a preview opened with ?redirect=false stays
// that way.
func backTo(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host {
		return "/"
	}
	if p := localPath(ref.RequestURI()); p != "" {
		return p
	}
	return "/"
}

// varyLang marks a response as depending on requestLang, so caches keep one
// copy per language.
func varyLang(w http.ResponseWriter) {
	w.Header().Add("Vary", "Accept-Language, Cookie")
}

// translation is a message in each language. Messages with arguments are
// fmt formats.
type translation struct {
	en, sv string
}

func (t translation) in(l lang) string {
	if l == swedish {
		return t.sv
	}
	return t.en
}

// tr is the message key in l, formatted with args if there are any.
func tr(l lang, key string, args ...any) (string, error) {
	m, ok := messages[key]
	if !ok {
		return "", fmt.Errorf("no message %q", key)
	}
	if len(args) == 0 {
		return m.in(l), nil
	}
	return fmt.Sprintf(m.in(l), args...), nil
}

// mustTr is tr for keys that are known to exist.
func mustTr(l lang, key string, args ...any) string {
	s, err := tr(l, key, args...)
	if err != nil {
		panic(err)
	}
	return s
}

// jsPrefix marks the messages swosh.js needs. They reach it as JSON, see
// jsMessages; {0} in them is filled in by the script.
const jsPrefix = "js."

// jsMessages are the script's messages in l, keyed without jsPrefix.
func jsMessages(l lang) map[string]string {
	out := map[string]string{}
	for k, m := range messages {
		if name, ok := strings.CutPrefix(k, jsPrefix); ok {
			out[name] = m.in(l)
		}
	}
	return out
}

// look is what a page is rendered in: a language and a theme.
type look struct {
	lang  lang
	theme theme
}

// parseTemplates parses the templates once per language and theme. Templates
// write {{t "key"}} for a message, {{lang}} for the language code and
// {{theme}} for the theme picked in the footer, empty for the system's.
func parseTemplates() map[look]*template.Template {
	sets := map[look]*template.Template{}
	for _, l := range langs {
		for _, th := range themes {
			sets[look{l, th}] = template.Must(template.New("").Funcs(template.FuncMap{
				"t":     func(key string, args ...any) (string, error) { return tr(l, key, args...) },
				"lang":  func() string { return string(l) },
				"theme": func() string { return string(th) },
				"js":    func() map[string]string { return jsMessages(l) },
			}).ParseFS(templateFiles, "templates/*.html"))
		}
	}
	return sets
}

// messages are the texts of the public pages and the API's error reasons.
var messages = map[string]translation{
	// Shared by all pages.
	"og.locale":       {"en_GB", "sv_SE"},
	"footer.source":   {"Swosh is open source on", "Swosh är öppen källkod på"},
	"footer.notswish": {"Swosh is an independent service, not part of Swish.", "Swosh är en fristående tjänst och inte en del av Swish."},
	// The switch offers the other language, named in that language.
	"lang.switch":      {"Svenska", "English"},
	"lang.switch.code": {"sv", "en"},
	"theme.light":      {"Light", "Ljust"},
	"theme.dark":       {"Dark", "Mörkt"},
	"github.title":     {"Swosh on GitHub", "Swosh på GitHub"},
	"create.link":      {"Create a link", "Skapa en länk"},

	// index.html
	"index.title":           {"Swosh – payment links for Swish", "Swosh – betallänkar för Swish"},
	"index.description":     {"Make a short link that opens Swish with your number, the amount and a message filled in.", "Gör en kort länk som öppnar Swish med ditt nummer, beloppet och ett meddelande ifyllt."},
	"index.eyebrow":         {"Payment links for Swish", "Betallänkar för Swish"},
	"index.heading":         {"Get paid back", "Få tillbaka pengarna"},
	"index.heading.accent":  {"in one tap", "med ett tryck"},
	"index.lead.before":     {"Make a short link that opens", "Gör en kort länk som öppnar"},
	"index.lead.after":      {"with your number, the amount and a message filled in. Send it to your friends: they tap it and confirm.", "med ditt nummer, beloppet och ett meddelande ifyllt. Skicka den till dina vänner: de trycker på den och bekräftar."},
	"index.payswith":        {"Your friends pay with Swish, in their own Swish app. Swosh only makes the link.", "Dina vänner betalar med Swish, i sin egen Swish-app. Swosh gör bara länken."},
	"form.phone":            {"Swish number", "Swish-nummer"},
	"form.phone.clear":      {"Clear the number", "Töm numret"},
	"form.recent":           {"Recent", "Senaste"},
	"form.amount":           {"Amount", "Belopp"},
	"form.message":          {"Message", "Meddelande"},
	"form.message.hint":     {"What's it for? (optional)", "Vad gäller det? (valfritt)"},
	"form.message.clear":    {"Clear the message", "Töm meddelandet"},
	"form.advanced":         {"Advanced", "Avancerat"},
	"form.expire":           {"Link expires after", "Länken slutar gälla efter"},
	"form.expire.10m":       {"10 minutes", "10 minuter"},
	"form.expire.30m":       {"30 minutes", "30 minuter"},
	"form.expire.1h":        {"1 hour", "1 timme"},
	"form.expire.12h":       {"12 hours", "12 timmar"},
	"form.expire.1d":        {"1 day", "1 dag"},
	"form.expire.1w":        {"1 week", "1 vecka"},
	"form.expire.1mo":       {"1 month", "1 månad"},
	"form.expire.6mo":       {"6 months", "6 månader"},
	"form.expire.never":     {"Never", "Aldrig"},
	"form.editable":         {"Can be changed in Swish", "Kan ändras i Swish"},
	"form.editable.number":  {"Number", "Nummer"},
	"form.editable.amount":  {"Amount", "Belopp"},
	"form.editable.message": {"Message", "Meddelande"},
	"form.editable.hint":    {"The payer can change these before paying. The rest is locked.", "Den som betalar kan ändra dessa innan betalningen. Resten är låst."},
	"form.reset":            {"Clear", "Töm"},
	"form.reset.title":      {"Clear the form", "Töm formuläret"},
	"form.error":            {"Couldn't create the link", "Kunde inte skapa länken"},
	"result.ready":          {"Your link is ready", "Din länk är klar"},
	"result.copy":           {"Copy", "Kopiera"},
	"result.share":          {"Share", "Dela"},
	"result.preview":        {"Preview", "Förhandsvisa"},
	"steps.fill":            {"Fill in the details", "Fyll i uppgifterna"},
	"steps.fill.text":       {"Your Swish number, the amount and what it's for.", "Ditt Swish-nummer, beloppet och vad det gäller."},
	"steps.share":           {"Share the link", "Dela länken"},
	"steps.share.text":      {"Drop it in the group chat, a text or an email.", "Släng in den i gruppchatten, ett sms eller ett mejl."},
	"steps.pay":             {"They tap and pay", "De trycker och betalar"},
	"steps.pay.text":        {"Swish opens with everything filled in. On a computer, they scan a QR code instead.", "Swish öppnas med allt ifyllt. På en dator skannar de en QR-kod i stället."},

	// swosh.js
	"js.forget":           {"Forget this number", "Glöm det här numret"},
	"js.forgetNumber":     {"Forget {0}", "Glöm {0}"},
	"js.expiresAfter":     {"Expires after {0}", "Slutar gälla efter {0}"},
	"js.neverExpires":     {"Never expires", "Slutar aldrig gälla"},
	"js.nothingEditable":  {"nothing editable", "inget kan ändras"},
	"js.editable":         {"{0} editable", "{0} kan ändras"},
	"js.and":              {"&", "&"},
	"js.missing":          {"Fill in the Swish number and the amount.", "Fyll i Swish-numret och beloppet."},
	"js.tooMuch":          {"You can Swish at most {0} kr.", "Du kan swisha högst {0} kr."},
	"js.wholeKronor":      {"The amount must be a whole number of kronor, at least 1.", "Beloppet måste vara ett helt antal kronor, minst 1."},
	"js.failed":           {"Something went wrong ({0}).", "Något gick fel ({0})."},
	"js.offline":          {"Could not reach Swosh. Check your connection and try again.", "Kunde inte nå Swosh. Kontrollera din anslutning och försök igen."},
	"js.expiresIn":        {"This link expires in {0}, on {1}.", "Länken slutar gälla om {0}, den {1}."},
	"js.neverExpiresLong": {"This link never expires.", "Länken slutar aldrig gälla."},
	"js.copied":           {"Copied!", "Kopierad!"},
	"js.copy":             {"Copy", "Kopiera"},

	// preview.html
	"preview.og.title":      {"Swish %dkr to %s", "Swisha %d kr till %s"},
	"preview.og.nomessage":  {"No message given...", "Inget meddelande..."},
	"preview.title":         {"Swish %d kr to %s · Swosh", "Swisha %d kr till %s · Swosh"},
	"preview.eyebrow":       {"Payment request", "Betalningsförfrågan"},
	"preview.to":            {"To", "Till"},
	"preview.message":       {"Message", "Meddelande"},
	"preview.expires":       {"Link expires", "Länken slutar gälla"},
	"preview.never":         {"Never", "Aldrig"},
	"preview.pay":           {"Pay with Swish", "Betala med Swish"},
	"preview.hint":          {"You check and confirm the payment in Swish.", "Du kontrollerar och bekräftar betalningen i Swish."},
	"preview.hint.editable": {"You check and confirm the payment in Swish, where you can change %s.", "Du kontrollerar och bekräftar betalningen i Swish, där du kan ändra %s."},
	"preview.qr.alt":        {"QR code to scan with Swish", "QR-kod att skanna med Swish"},
	"preview.qr":            {"On a computer? Open Swish on your phone and scan the code.", "Sitter du vid en dator? Öppna Swish i telefonen och skanna koden."},
	"preview.own":           {"Want to get paid back too?", "Vill du också få tillbaka pengar?"},
	"preview.own.link":      {"Make your own Swosh link", "Gör din egen Swosh-länk"},
	"editable.number":       {"the number", "numret"},
	"editable.amount":       {"the amount", "beloppet"},
	"editable.message":      {"the message", "meddelandet"},
	"editable.and":          {"and", "och"},

	// notfound.html
	"notfound.title": {"Link not found", "Länken finns inte"},
	"notfound.text":  {"This Swosh link doesn't exist or has expired. Ask whoever sent it for a new one.", "Den här Swosh-länken finns inte eller har slutat gälla. Be den som skickade den om en ny."},
	"notfound.new":   {"Create a new Swosh", "Skapa en ny Swosh"},

	// login.html and logout.html
	"login.title":     {"Sign in", "Logga in"},
	"login.heading":   {"Admin sign in", "Logga in som admin"},
	"login.text":      {"Sign in to manage Swosh links.", "Logga in för att hantera Swosh-länkar."},
	"login.loggedout": {"You have been logged out.", "Du har loggats ut."},
	"login.bad":       {"Bad credentials", "Fel användarnamn eller lösenord"},
	"login.username":  {"Username", "Användarnamn"},
	"login.password":  {"Password", "Lösenord"},
	"login.submit":    {"Sign in", "Logga in"},
	"login.back":      {"Back to Swosh", "Tillbaka till Swosh"},
	"logout.title":    {"Log out", "Logga ut"},
	"logout.heading":  {"Log out?", "Logga ut?"},
	"logout.text":     {"You will have to sign in again to get back to the admin pages.", "Du måste logga in igen för att komma tillbaka till adminsidorna."},
	"logout.back":     {"Back", "Tillbaka"},
	"logout.submit":   {"Log out", "Logga ut"},

	// POST /api/create. The English ones are the Kotlin service's.
	"api.badinput":    {"Invalid input format!", "Kunde inte läsa förfrågan."},
	"api.failed":      {"Unable to generate Swosh!", "Kunde inte skapa länken."},
	"api.missing":     {"Missing input parameters. 'phone' and 'amount' is required", "Swish-nummer och belopp måste fyllas i."},
	"api.min":         {"Minimum allowed amount is 1. Got %d", "Beloppet måste vara minst 1 kr, inte %d."},
	"api.max":         {"Maximum allowed amount is %d. Got %d", "Beloppet får vara högst %d kr, inte %d."},
	"api.toolong":     {"Description is too long. Max %d chars. Got %d", "Meddelandet är för långt: högst %d tecken, men det har %d."},
	"api.badphone":    {"'%s' is not a valid phone number", "'%s' är inte ett giltigt telefonnummer."},
	"api.notmobile":   {"'%s' is not a mobile number", "'%s' är inte ett mobilnummer."},
	"api.needmessage": {"A message is required for links that expire after 6 months or more, or never", "Länkar som gäller i 6 månader eller längre, eller aldrig slutar gälla, måste ha ett meddelande."},
	"api.badexpiry":   {"'expireAfterSeconds' must be between 0 and %d. Got %d", "'expireAfterSeconds' måste vara mellan 0 och %d, inte %d."},
}
