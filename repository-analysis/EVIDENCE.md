# Evidence and review notes

## Scope and screening

**Level: full.** Receipt-specific OCR, multimodal structured extraction, batch orchestration, and HTML reporting constitute meaningful original implementation beyond the initial scaffold. This is not a recommendation to feature or publish the project.

Inspected local `main`, `origin/main`, and symbolic `origin/HEAD`; all resolve to `f20015c78224ca9e378590fd24b04c48997bc42f`. No tags or other local branches were available. All 46 reachable commits were enumerated, with representative initial and feature changes inspected. No remotes were fetched. Working tree was initially clean and no prior `repository-analysis/` existed. No applicable AGENTS.md was found in the repository. The external analysis prompt was read as requested.

Sources included root README, command documentation, `go.mod`, configuration loader and example config, `.gitignore`, Dockerfile, receipt commands, all OCR and LLM files, OpenAI request/schema/polling/metadata code, and the structure/imports of scaffold email and middleware packages. The committed report PNG was inventoried but not visually reviewed or approved for public reuse. No tracked tests or CI workflows were found. Source was inspected statically; no project programs, tests, dependency installation, API calls, or container builds were executed. Runtime operability and current provider compatibility are unverified. No external research was necessary for repository-local claims.

## Claim ledger

| Claim | Evidence | Confidence |
| --- | --- | --- |
| The user's recollection of supermarket images producing category reports through OpenAI is supported. | `src/cmd/receipt-pipeline/main.go:processOneImage`; `src/pkg/llm/analyze-receipt-image.go:GenerateReceiptAnalysisFromImage`; `src/cmd/report/main.go:buildMonthlyReport`. | High: explicit connected implementation. Actual usage is unverified. |
| OCR includes preprocessing and separate text/numeric passes. | `src/pkg/ocr/preprocess.go:createProcessedImage`; `tesseract.go:runOcrOnImage`, `runOcrForNumbers`; `pipeline.go:ProcessImage`; `regex.go:ExtractPriceCandidates`. Commits `db1785a`, `441b44d`, `a985ced`, `a659a2b`. | High: direct source and history. Accuracy benefits are unmeasured. |
| The original image, OCR text and price hints are sent together with strict JSON Schema output requested. | `src/pkg/llm/analyze-receipt-image.go`; `src/pkg/openai/generic-use-openai-w-image.go:UseChatGPTResponsesAPIWithImage`; `text-format-builders.go:TextAsJSONSchema`. Commit `9e29028`. | High: request construction, not proof of successful live execution. |
| Current extraction targets COP and hard-codes gpt-5-mini with low reasoning effort. | `src/pkg/llm/analyze-receipt-image.go:GenerateReceiptAnalysisFromImage`; `src/pkg/config/init.go:Config`. Commit `bc828a4`. | High. Root README's claim that model selection lives in config is contradicted by the code. |
| The batch command sorts supported files in one directory and continues on per-file errors. | `src/cmd/receipt-pipeline/main.go:listImagesInDir`, `resolveImagesToProcess`, main loop. Commits `9b8c265`, `414681f`. | High: nonrecursive directory input; no concurrency or resume mechanism. |
| Original/processed images, OCR text, numeric text, price JSON and analysis JSON are retained per run. | `src/pkg/ocr/pipeline.go:ProcessImage`; `src/cmd/receipt-pipeline/main.go:processOneImage`; `src/pkg/openai/build-metadata.go:ExtractLLMRunMetadata`. | High: explicit file writes and metadata attachment. |
| Report generation recursively reads receipt-analysis.json files and produces monthly category HTML. | `src/cmd/report/main.go:collectJSONFiles`, `buildMonthlyReport`, `chooseReceiptTotal`, `buildCategoryRows`, `renderHTML`. Commits `40a0d6f`, `f20015c`. | High: implementation present; no fixture-based verification. |
| Normal generated data is grouped by processing date rather than purchase date. | `src/pkg/llm/analyze-receipt.go:ReceiptAnalysis` and image schema omit dates; `src/cmd/report/main.go:determineReceiptTime` falls back to metadata `started_at`; metadata populated in `src/pkg/openai/build-metadata.go`. | High: producer/consumer schema comparison. Manually supplied explicit dates can override this. |
| Total checking is model-derived, optional, and disabled by default. | `src/cmd/receipt-pipeline/main.go:processOneImage`, `price-difference` flag; image-analysis prompt. Commit `dcd1022`. | High: acceptance depends on total_check_message rather than a Go sum check. |
| Receipt workflow uses filesystem storage; web/email dependencies are not its architecture. | Receipt command import paths; `go.mod`; `src/pkg/email`, `src/pkg/echo-middleware`, `src/cmd/send-email/main.go`; initial commit `a5d195c`. | High for separation; medium for scaffold origin. These packages existed before receipt-specific commits. |
| Receipt-specific work was added after an apparent reusable starter layout. | `a5d195c` adds generic example commands, email/middleware/OpenAI utilities and placeholder Dockerfile; `6f404ed` renames this-project; `src/pkg/config/init.go` retains project-layout naming. | Medium: template indicators are strong, but no upstream template/fork identity is established. Do not claim authorship of all starter code. |

## Timeline basis

Git author and committer dates agree at the activity endpoints: earliest `a5d195c`, 2025-11-26T09:14:15-05:00; latest `f20015c`, 2026-01-30T21:48:18-05:00. Dates in metadata preserve day precision. One intermediate commit has a small author/committer time difference, without affecting these endpoints.

- **2025-11-26:** initial scaffold (`a5d195c`), OCR command (`db1785a`), separated OCR package (`e265da1`), analysis implementation/command (`3454263`, `cc80d1d`), month directories (`a16001d`).
- **2025-12-11:** model switch (`bc828a4`), numeric OCR and price hints (`441b44d`, `a985ced`, `066dc4e`), image-based analysis (`9e29028`).
- **2025-12-13:** first report implementation (`40a0d6f`).
- **2026-01-04:** orientation (`a659a2b`), combined command (`9b8c265`), directory batches (`414681f`), fractional report quantities (`2cb9fbc`).
- **2026-01-10–30:** README, prompts, logging, dependency/toolchain, category, optional mismatch-check and report row changes (`a950baf`, `bc051ac`, `e3162d9`, `3b9415a`, `dcd1022`, `f20015c`).

Activity clusters have gaps, including November 26 to December 11 and December 13 to January 4. Neither continuous development nor actual project start/end dates can be inferred. No activity after January 30 is visible in available refs; this does not establish abandonment.

## Uncertainties requiring owner review

1. Personal/professional/educational context and ownership or provenance of initial scaffold/support packages; project type remains unknown.
2. Actual operational state, usage, deployments, receipt volume, extraction accuracy, API cost, and time savings. No verified metrics support résumé claims.
3. Intended handling of purchase dates, reprocessed receipts, discounts, currencies and discrepancies; current COP-specific behavior may be narrower than future intent.
4. Repository URL is derived from the configured Git remote; accessibility and public visibility are unverified. Relationship is provisionally standalone based on the self-contained workflow; no parent project is identified.
5. Actual project start/end dates remain unknown. The owner confirmed on 2026-10-15 that development is currently paused; metadata uses `inactive` because the required schema has no `paused` status. The pause start date is unknown. Contribution wording describes evidenced repository work; exclusive personal authorship requires confirmation.

## Publication risks

Receipt originals, OCR, model outputs and reports can expose financial/purchase data and identifiers. Output directories are ignored, but this is not a privacy guarantee. The committed screenshot needs owner privacy review before reuse; the proposed README intentionally omits it. API request code sends the original image and OCR externally and sets `store: true` and `background: true` in `src/pkg/openai/run.go`; local artifact retention must not be described as offline processing. Debug logging includes request payloads, and analysis logging includes purchase details. Response identifiers/log links are stored; do not publish real run data without review.

Environment example files are tracked; configuration and non-example environment files are ignored. This analysis is not a complete secret audit of all historical blobs, and no credential values are copied into these artifacts. Generic mail/example code may contain personal or deployment context and needs review before broad publication. No root license file was found; code reuse rights and scaffold provenance need clarification. `publication.show_criticism` remains false; that controls portfolio display, not GitHub file visibility.

## Owner clarification

On 2026-10-15, the repository owner stated that the project is currently on pause. This supplements Git evidence and establishes current status, not a completion date or the date the pause began. Public documentation records the pause; `project.status` is `inactive` under the prescribed schema.
