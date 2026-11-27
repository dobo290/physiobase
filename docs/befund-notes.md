# Assessment form (Befund) – notes and open improvements

Working list for the assessment form only. The overall plan lives in `ROADMAP.md`;
this file collects the small and medium improvements that come from using the form
in the practice. Each entry says what happens today, what should happen, and where
the code is.

Status: `[ ]` open · `[x]` done. When an entry is released, move it to the Done
section of `ROADMAP.md` and delete it here.

---

## 1. Rename the section "Assessments – Notizen" to "Tests/Assessments"

- **Today:** the free-text section after the PROM table is titled
  "Assessments – Notizen", which reads like a note field below the assessments.
- **Should be:** "Tests/Assessments" – a place for findings from tests and
  assessments that do not fit into the value table.
- **Where:** `internal/befund/schema.go` (section title), nothing else depends on the
  title.
- **Watch out:** the section keeps its key (`assess`), so existing findings are
  unaffected by the rename.

## 2. Print header shows an age entered as "45" as "geb. 45"

- **Today:** the patient's `dob` field accepts either a date or an age
  (placeholder "Geburtsdatum / Alter"). Everything is printed with the prefix
  "geb. ", so an age of 45 appears as "geb. 45" – wrong, it is not a date.
- **Should be:** distinguish the three cases and label them correctly:
  | Input | Printed as |
  |---|---|
  | `12.04.1969` | geb. 12.04.1969 |
  | `1969` (year) | geb. 1969 |
  | `45` / `45 J.` (age) | 45 Jahre |
- **Where:** `web/templates/befund_print.html` (print header), and the same
  expression in `web/templates/befund.html` and `web/templates/patient.html` so the
  screen matches the printout. Rule belongs in Go (a template helper), not in three
  templates: `dobLabel(dob string) string`.
- **Watch out:** `TestPagesRender` and the patient tests use `dob` values like
  `1965`; the helper must leave them as a year.

## 3. Pain subtype column is printed even when it is empty

- **Today:** the pain table prints every column of the schema. "Subtyp (nur
  nozizeptiv)" therefore appears as an empty column whenever no row has a subtype,
  which is the normal case.
- **Should be:** a column is only printed when at least one row has a value in it.
  This applies to the whole table, not just the pain table: empty "Bemerkung"
  columns disappear as well. The input form keeps all columns – only the printout
  and the saved findings view are trimmed.
- **Where:** `internal/befund/data.go` (new helper, e.g.
  `func (d Data) FilledCols(t *Table) []Col`), used in
  `web/templates/befund_print.html` and the summary/print views that render tables.
- **Watch out:** the comparison view (Δ table) must keep the column, otherwise two
  assessments with different filled columns cannot be compared.

## 4. Inspektion: additional field "Subjektive Wahrnehmung (Suggestiv)"

- **Today:** the inspection section has one free-text field
  ("Inspektionsbefund (Haltung, Haut, Ödeme, Narben, Atmung etc.)").
- **Should be:** a second, short field "Subjektive Wahrnehmung (Suggestiv)" next to
  it, for the impression that the patient's own description suggests – kept apart on
  purpose, so the objective finding and the subjective impression do not blur.
- **Where:** `internal/befund/schema.go`, section `inspektion` (new field key, for
  example `insp_subj`, single line or a small text area).
- **Watch out:** a new key means existing findings simply have no value there (fields
  render only when filled), so no migration is needed. Keep it out of the objective
  field so the printout stays distinguishable: the label is the separator.

---

## Adding an entry

One section per item, with: what happens today, what should happen, where the code
is, and what could break. Keep it short enough that the next person can implement it
without asking.
