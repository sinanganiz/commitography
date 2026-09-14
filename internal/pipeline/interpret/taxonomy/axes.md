# Interpretation axes

Governed by ADR-0013, ADR-0014, ADR-0030.

Archetype and badge conditions in `taxonomy.yml` are written against **named
axes**, never against raw report paths. The mapping from axis to metric lives
here and in one place in code.

The reason is maintenance, not style. If a condition reads
`messages.conventional_ratio >= 0.85`, then every threshold breaks when that
metric moves or is renamed, and the taxonomy becomes coupled to the report
schema. With an axis layer, a metric change updates one mapping, and
`taxonomy.yml` stays a document about people rather than a document about
JSON paths.

An axis is either a scalar normalised to the range 0 to 1, a raw count, or a
boolean. Ranks are 1-based and computed **within the repository**, which is the
second comparison axis in ADR-0013. A rank axis is unavailable when the
repository has fewer than two identities; a badge depending on one is then
reported skipped, never awarded by default.

---

## Person axes

| Axis | Source | Form |
|---|---|---|
| `nocturnality` | `temporal.night_ratio`, person scope | 0–1 |
| `earliness` | `temporal.early_ratio`, person scope | 0–1 |
| `weekend_presence` | `temporal.weekend_ratio`, person scope | 0–1 |
| `friday_evening_share` | `temporal.friday_evening_count` ÷ person commits | 0–1 |
| `regularity` | `temporal.regularity`, person scope | 0.25–1 |
| `cadence` | `temporal.commits_per_active_day`, person scope | raw |
| `endurance` | person active days ÷ `temporal.repository_age_days` | 0–1 |
| `personal_span_days` | last minus first person commit, whole days | raw |
| `granularity` | `commit_size.small_commit_ratio`, person scope | 0–1 |
| `breadth` | person `mean_files` ÷ repository `p90_files`, capped at 1 | 0–1 |
| `creation` | `worktype` `new_work` share, person projection | 0–1 |
| `restoration` | `worktype` `legacy_refactor` share, person projection | 0–1 |
| `revision` | `worktype` `rework` share, person projection | 0–1 |
| `collaboration` | `worktype` `help_others` share, person projection | 0–1 |
| `concentration` | largest share of the person's surviving lines in any one depth-1 directory | 0–1 |
| `reach` | depth-1 directories touched ÷ depth-1 directories present | 0–1 |
| `persistence` | person's surviving lines ÷ lines they authored | 0–1 |
| `volume_share` | person commits ÷ analysed commits | 0–1 |
| `formality` | `messages.conventional_ratio`, person scope | 0–1 |
| `verbosity` | normalised blend of mean subject length and body ratio, person scope | 0–1 |
| `fix_share` | `fix` classified ÷ person commits | 0–1 |
| `new_file_share` | person commits introducing a new path ÷ person commits | 0–1 |
| `assistance` | `ai_archaeology` assisted ÷ person commits | 0–1 |
| `first_contributor` | person authored the first analysed commit | boolean |
| `most_recent_contributor` | person authored the most recent analysed commit | boolean |
| `repo_assistance` | repository `ai_archaeology.assisted_commit_ratio` | 0–1 |
| `repo_formality` | repository `messages.conventional_ratio` | 0–1 |
| `identity_count` | individually represented identities | raw |
| `rank_*` | 1-based rank of the person within the repository on the named axis | raw |

## Repository axes

| Axis | Source |
|---|---|
| `nocturnality`, `weekend_presence`, `office_hours`, `regularity`, `span_ratio` | `temporal` |
| `longest_silence_days`, `repository_age_days` | `temporal` |
| `days_since_last_commit` | analysed commit date minus `temporal.last_commit_date` |
| `bus_factor`, `knowledge_concentration`, `median_line_age_days` | `ownership` |
| `identity_count`, `directory_count` | `ownership`, `files` |
| `revision` | `worktype.repository_shares.rework` |
| `coupled_file_ratio` | `coupling` |
| `repo_assistance` | `ai_archaeology.assisted_commit_ratio` |


## Badge axes

Badges use raw counts more often than normalised scalars, because a badge is an
event or a superlative rather than a tendency. All are person-scoped unless
stated.

| Axis | Source |
|---|---|
| `commits` | Analysed commits by the identity set |
| `active_days` | Distinct active dates, person scope |
| `streak_days` | `temporal.longest_streak_days`, person scope |
| `longest_personal_silence_days` | Largest gap between the person's adjacent commits |
| `commits_after_return` | Commits after the person's longest personal silence |
| `single_line_commits` | Commits with effective lines equal to 1 |
| `largest_commit_lines` | Effective lines of the person's largest non-bulk commit |
| `deletion_ratio` | Lines removed ÷ effective lines, person scope |
| `new_files_created` | Non-excluded paths first introduced by the person |
| `single_file_ratio` | `commit_size.single_file_ratio`, person scope |
| `mean_subject_length` | `messages.mean_subject_length`, person scope |
| `very_short_subjects` | `messages.very_short_count`, person scope |
| `emoji_ratio` | `messages.emoji_ratio`, person scope |
| `revert_commits` | `messages.revert_count`, person scope |
| `fix_typo_commits` | `messages.fix_typo_count`, person scope |
| `friday_evening_commits` | `temporal.friday_evening_count`, person scope |
| `commits_in_hour_0` | `temporal.hour_histogram[0]`, person scope |
| `commits_on_dec_25_or_jan_1` | Commits on 25 December or 1 January, author local date |
| `commits_on_feb_29` | Commits on 29 February, author local date |
| `max_commits_in_one_minute` | Largest count sharing one author-local minute |
| `sole_owner_of_directories` | Directories in `ownership` scope where the person holds every surviving line |
| `owns_oldest_untouched_file` | The person authored the last change to `files.oldest_untouched` |
| `owns_top_coupled_pair` | The person touched both files of the highest-support coupling pair more than anyone else |
| `owns_top_hotspot` | The person holds the largest surviving-line share of the top `hotspot.hotspots` entry |
| `first_assisted_commit` | The person authored `ai_archaeology.first_assisted_commit_date` |

A badge whose source family is `skipped` is itself skipped with that family's
reason, never awarded and never silently dropped (ADR-0032).

---

## Calibration

Thresholds in `taxonomy.yml` are hand-set. They are not derived from a
population and must never be presented as a percentile or a ranking against
other users (ADR-0013 clause 3).

Two properties are enforced by test (ADR-0056):

1. **Reachability.** Every archetype in both lists is reached by at least one
   fixture. An archetype that no fixture reaches is either mis-ordered or
   mis-thresholded.
2. **Default description.** Every archetype has a non-empty description, so the
   product is complete with no language model configured (ADR-0039 clause 5).

A third property is recommended and not enforced: no single archetype should
absorb a large majority of fixture subjects. A taxonomy whose output is almost
always the fallback is calibrated too tightly; one that is almost always a single
distinctive archetype is calibrated too loosely.

## Ordering

The person list is ordered by specificity: temporal signatures first, then
tempo, then shape of change, then role in the codebase, then message style,
then assistance, then the fallback. A rule placed above another can starve it,
which is why reachability is a test rather than a hope.

When adding an archetype, place it above every archetype whose conditions it is
a special case of, and add a fixture that reaches it in the same change.
