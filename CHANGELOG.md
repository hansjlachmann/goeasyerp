# Changelog

## [0.1.147](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.146...v0.1.147) (2026-11-09)


### Features

* G/L balance and budget views ([fedeae7](https://github.com/hansjlachmann/goeasyerp/commit/fedeae76d2ae140595ae942d8cf7fe53253f6ca4))
* journal reconcile, batch posting, test reports and line lists ([3aed8f6](https://github.com/hansjlachmann/goeasyerp/commit/3aed8f68bcabfa319de724b9771202e097fbfaf7))
* links from master data, documents and registers ([ba47479](https://github.com/hansjlachmann/goeasyerp/commit/ba474799e886a43fb82565888fccfd1e770447b3))
* statistics pages and applied entries ([88a3ad6](https://github.com/hansjlachmann/goeasyerp/commit/88a3ad6fbbad977254294bfba2de9f9fe9e9ae16))

## [0.1.146](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.145...v0.1.146) (2026-11-09)


### Features

* Combine Shipments, Delete Invoiced orders, Index fixed assets/insurance and the BOM reports ([31108e0](https://github.com/hansjlachmann/goeasyerp/commit/31108e0d13973ef0a4e778a63287def907f00136))
* Copy Document on sales and purchase documents, and the customer credit limit warning ([ed1a8f0](https://github.com/hansjlachmann/goeasyerp/commit/ed1a8f05c6536c477a0eb44c1a9f72e2802f778a))
* Explode BOM on sales and purchase lines ([ea188ec](https://github.com/hansjlachmann/goeasyerp/commit/ea188eca5a9c4c936758eabb8030f5f3ff891908))
* extended texts on documents ([70b3462](https://github.com/hansjlachmann/goeasyerp/commit/70b3462f3b9b09eeafb15526c6e5c8257bdc8d31))
* item availability warning on sales lines and journals ([f20028b](https://github.com/hansjlachmann/goeasyerp/commit/f20028b614fd23a9600854c62fcf78211b94fb38))
* statistics of posted shipments, receipts, invoices and credit memos ([3b330a6](https://github.com/hansjlachmann/goeasyerp/commit/3b330a61a72b9fdf4e83dd36ef2de49048f45863))


### Bug Fixes

* a line field's function runs on the saved line and the lines show what it changed ([073396f](https://github.com/hansjlachmann/goeasyerp/commit/073396f6e3d38e7a30b2e6a4479063f088252091))


### CI/CD

* allow the backend tests 30 minutes ([0820d51](https://github.com/hansjlachmann/goeasyerp/commit/0820d510ac9bd7bae405dee75b5acc3eb5083a6e))

## [0.1.145](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.144...v0.1.145) (2026-11-08)


### Features

* a row menu on lists (right-click, Shift+F10) and an Actions menu ([b1a7db7](https://github.com/hansjlachmann/goeasyerp/commit/b1a7db7cf2ae451625714888a8c7269fdf49100d))


### Bug Fixes

* show the number series warning, translate the rate-limit message, relate Customer Posting Group ([33a7a4d](https://github.com/hansjlachmann/goeasyerp/commit/33a7a4db083ba857ff709bb73ded9e46fb9d2060))

## [0.1.144](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.143...v0.1.144) (2026-11-08)


### Features

* store page layouts on the server, so they follow the user ([33ad3ef](https://github.com/hansjlachmann/goeasyerp/commit/33ad3ef0b12fb52e87878711ac56950b3945520c))


### Bug Fixes

* align numbers right in lists, column captions, footers and cards ([6ece848](https://github.com/hansjlachmann/goeasyerp/commit/6ece8482a53c0cee7dfb44f35f3cf7135354bd65))
* keep a list's filters and FlowFilters when coming back to it ([b37e17c](https://github.com/hansjlachmann/goeasyerp/commit/b37e17c2c195c60072b0cf926346d92a41827885))

## [0.1.143](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.142...v0.1.143) (2026-11-08)


### Bug Fixes

* drill down with the FlowField's own filters on every summed column ([86bcfaf](https://github.com/hansjlachmann/goeasyerp/commit/86bcfaffbdd57c34c076b218646e0f308553aab3))

## [0.1.142](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.141...v0.1.142) (2026-11-08)


### Features

* refuse deleting records that are still in use ([6586968](https://github.com/hansjlachmann/goeasyerp/commit/658696828de2c9bcaa3b23ee5a9978e2b6578d91))


### Bug Fixes

* drill down on FlowFields with their own filters, so total accounts show their Totaling range ([ea855c2](https://github.com/hansjlachmann/goeasyerp/commit/ea855c2226eee2a16efd5a6467c8fc6f52e7b698))
* generate keyed delete-rule references so vet passes ([8ab1eff](https://github.com/hansjlachmann/goeasyerp/commit/8ab1eff37ca1818b2e4b07f029d73e34fa880bf5))

## [0.1.141](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.140...v0.1.141) (2026-11-07)


### Bug Fixes

* action bar wraps, aligns with the list grid and groups functions into menus ([2a10068](https://github.com/hansjlachmann/goeasyerp/commit/2a1006849793040c2a1a2cdb59279f7260c9f8f1))
* list header shortcuts, caption wrapping, amount decimals and demo data gaps ([c54129c](https://github.com/hansjlachmann/goeasyerp/commit/c54129cd88bfa33906aa8c610a4fe568d534f598))
* menu button keys, card edge buttons, whole-word captions and unknown journal batches ([a2e0a52](https://github.com/hansjlachmann/goeasyerp/commit/a2e0a52efd723dc0578a6e238783b38fec3cfb50))

## [0.1.140](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.139...v0.1.140) (2026-11-06)


### Features

* apply posted customer and vendor entries, and unapply applications ([#236](https://github.com/hansjlachmann/goeasyerp/issues/236)) ([636caad](https://github.com/hansjlachmann/goeasyerp/commit/636caad2be1661234ca7b82fa93459791dcef1a6))

## [0.1.139](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.138...v0.1.139) (2026-11-05)


### Features

* Object Caption, CaptionClass and printed reports fitting the brand fonts ([#234](https://github.com/hansjlachmann/goeasyerp/issues/234)) ([7b4bf0d](https://github.com/hansjlachmann/goeasyerp/commit/7b4bf0d79d9e0542c08ea7f096799f53e15057c0))


### Bug Fixes

* FlowFields on a global table (Report Selections' Report Name on ([7b4bf0d](https://github.com/hansjlachmann/goeasyerp/commit/7b4bf0d79d9e0542c08ea7f096799f53e15057c0))

## [0.1.138](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.137...v0.1.138) (2026-11-04)


### Features

* run the demo data (codeunit 901) again in the same company ([#231](https://github.com/hansjlachmann/goeasyerp/issues/231)) ([c29058e](https://github.com/hansjlachmann/goeasyerp/commit/c29058e37323ce29613b3bc65beeac7cd93ffd0b))


### Bug Fixes

* the LARGE and HEAVY demo could not post its sales documents: the ([c29058e](https://github.com/hansjlachmann/goeasyerp/commit/c29058e37323ce29613b3bc65beeac7cd93ffd0b))


### Documentation

* Windows installation with Docker Desktop in the README ([#232](https://github.com/hansjlachmann/goeasyerp/issues/232)) ([d70b9a2](https://github.com/hansjlachmann/goeasyerp/commit/d70b9a2c0186f6191d9a3ee6b55ba20b17156bcb))

## [0.1.137](https://github.com/hansjlachmann/goeasyerp/compare/v0.1.136...v0.1.137) (2026-11-04)


### Features

* single-file installation for trying GoEasyERP on one computer ([#229](https://github.com/hansjlachmann/goeasyerp/issues/229)) ([66019d2](https://github.com/hansjlachmann/goeasyerp/commit/66019d277ee33d579c254605aeb59ff765194fd1))

## [0.1.136](https://github.com/hansjlachmann/openerp/compare/v0.1.135...v0.1.136) (2026-11-04)


### Features

* journal test reports and Post and Print ([#225](https://github.com/hansjlachmann/openerp/issues/225)) ([486f0cf](https://github.com/hansjlachmann/openerp/commit/486f0cf246c0f31994549c09cf1529c84edb3d29))

## [0.1.135](https://github.com/hansjlachmann/openerp/compare/v0.1.134...v0.1.135) (2026-11-04)


### Features

* object identity — every business-logic object in the Object table ([#222](https://github.com/hansjlachmann/openerp/issues/222)) ([562f840](https://github.com/hansjlachmann/openerp/commit/562f840f4d6f5dc225eca86d132fb635bf2a1801))

## [0.1.134](https://github.com/hansjlachmann/openerp/compare/v0.1.133...v0.1.134) (2026-11-04)


### Bug Fixes

* check the CalcSums error in the Get Project Usage test (errcheck) ([#220](https://github.com/hansjlachmann/openerp/issues/220)) ([130cbf9](https://github.com/hansjlachmann/openerp/commit/130cbf926578d8796dd9dbfdaeee69221493146f))

## [0.1.133](https://github.com/hansjlachmann/openerp/compare/v0.1.132...v0.1.133) (2026-11-04)


### Features

* applied project entries, reconciliation and the project journal test report ([#214](https://github.com/hansjlachmann/openerp/issues/214)) ([ac085f3](https://github.com/hansjlachmann/openerp/commit/ac085f3eefa5add724c4660412e9f82e7643164a))
* FA book value reports on the report engine's group sections ([#216](https://github.com/hansjlachmann/openerp/issues/216)) ([617fefa](https://github.com/hansjlachmann/openerp/commit/617fefad05df660aac9dc94b8d123b87ada5a7c3))
* log of refused postings for administrators ([#215](https://github.com/hansjlachmann/openerp/issues/215)) ([acfde67](https://github.com/hansjlachmann/openerp/commit/acfde67064b7fb5b8c5a4acd0b1ce150de9f411a))
* project resource availability, budget date changes and project usage on invoices ([#218](https://github.com/hansjlachmann/openerp/issues/218)) ([078be86](https://github.com/hansjlachmann/openerp/commit/078be864ffc47bf268a52c24e6668c749545f431))
* the job queue runs reports ([#217](https://github.com/hansjlachmann/openerp/issues/217)) ([5d89ddc](https://github.com/hansjlachmann/openerp/commit/5d89ddc8e85238a2f8c575b60a55e20f3e7b02c8))

## [0.1.132](https://github.com/hansjlachmann/openerp/compare/v0.1.131...v0.1.132) (2026-11-03)


### Features

* group headers and footers in the report engine ([#211](https://github.com/hansjlachmann/openerp/issues/211)) ([7143bf2](https://github.com/hansjlachmann/openerp/commit/7143bf270abf4a959822e94e2934f1446f96e97f))


### Bug Fixes

* pages opened from a modal page stack over it; Esc goes back the way pages were opened ([#212](https://github.com/hansjlachmann/openerp/issues/212)) ([c371241](https://github.com/hansjlachmann/openerp/commit/c371241bcbb44e98d9fbbd25afc7bc6317574a93))

## [0.1.131](https://github.com/hansjlachmann/openerp/compare/v0.1.130...v0.1.131) (2026-11-03)


### Features

* project statistics, WIP/recognition, budget copy and project reports (Projects P3) ([#209](https://github.com/hansjlachmann/openerp/issues/209)) ([7e95409](https://github.com/hansjlachmann/openerp/commit/7e954094894d93fa9389ac62ec0241bc178a7562))
* projects module P2 — project journal, ledger and posting; Purchases menu ([#208](https://github.com/hansjlachmann/openerp/issues/208)) ([1188ca8](https://github.com/hansjlachmann/openerp/commit/1188ca817b04c85534c6ae332c119da6d9c1613d))

## [0.1.130](https://github.com/hansjlachmann/openerp/compare/v0.1.129...v0.1.130) (2026-11-03)


### Features

* generic dimensions — Department/Project become Dimension 1/2, named in General Ledger Setup ([#205](https://github.com/hansjlachmann/openerp/issues/205)) ([f154d9d](https://github.com/hansjlachmann/openerp/commit/f154d9d3ce50b7e488a16c8dca094d10877f702b))

## [0.1.129](https://github.com/hansjlachmann/openerp/compare/v0.1.128...v0.1.129) (2026-11-03)


### Features

* bins, Item Entry-Edit and the shipment/receipt line checks ([#202](https://github.com/hansjlachmann/openerp/issues/202)) ([4273ea2](https://github.com/hansjlachmann/openerp/commit/4273ea2c0c0efc3e1de57cb9ca9e834a420357e7))

## [0.1.128](https://github.com/hansjlachmann/openerp/compare/v0.1.127...v0.1.128) (2026-11-03)


### Features

* Preview Posting on the item, resource, FA, BOM and insurance journals ([#200](https://github.com/hansjlachmann/openerp/issues/200)) ([d78e592](https://github.com/hansjlachmann/openerp/commit/d78e592639d7c111fc7ec38bade115b8d889f9e0))

## [0.1.127](https://github.com/hansjlachmann/openerp/compare/v0.1.126...v0.1.127) (2026-11-03)


### Bug Fixes

* Post Inventory Cost to G/L texts keep their values ([#193](https://github.com/hansjlachmann/openerp/issues/193)) ([5049776](https://github.com/hansjlachmann/openerp/commit/5049776063ed1b8a8763221559160587aef76522))

## [0.1.126](https://github.com/hansjlachmann/openerp/compare/v0.1.125...v0.1.126) (2026-11-02)


### Features

* report selections, printer selections, item price changes and requisition worksheets ([#190](https://github.com/hansjlachmann/openerp/issues/190)) ([47a04a9](https://github.com/hansjlachmann/openerp/commit/47a04a9ea25bd1eeae7cc8aa624146dcbe5a7771))

## [0.1.125](https://github.com/hansjlachmann/openerp/compare/v0.1.124...v0.1.125) (2026-11-02)


### Features

* account schedules with overview and printout ([#187](https://github.com/hansjlachmann/openerp/issues/187)) ([7dee7c1](https://github.com/hansjlachmann/openerp/commit/7dee7c16221ecfab9dc9e71c983a1e355cb54c9f))

## [0.1.124](https://github.com/hansjlachmann/openerp/compare/v0.1.123...v0.1.124) (2026-11-02)


### Features

* G/L budgets with a budget matrix by periods ([#185](https://github.com/hansjlachmann/openerp/issues/185)) ([78c91c3](https://github.com/hansjlachmann/openerp/commit/78c91c34ea553727127597136ed9408f9430249f))

## [0.1.123](https://github.com/hansjlachmann/openerp/compare/v0.1.122...v0.1.123) (2026-11-01)


### Features

* fixed asset reclassification, insurance, printouts, posted demo data and demo journals (FA5, FA6) ([#179](https://github.com/hansjlachmann/openerp/issues/179)) ([44f3305](https://github.com/hansjlachmann/openerp/commit/44f3305f676a07f5a8e8e4c65dd3d9d1b0943f48))

## [0.1.122](https://github.com/hansjlachmann/openerp/compare/v0.1.121...v0.1.122) (2026-11-01)


### Features

* purchase history and demo purchases (P4, P5) ([#173](https://github.com/hansjlachmann/openerp/issues/173)) ([cdcf594](https://github.com/hansjlachmann/openerp/commit/cdcf5941d7aebcc2cf1340e798fbcb022f409218))

## [0.1.121](https://github.com/hansjlachmann/openerp/compare/v0.1.120...v0.1.121) (2026-10-29)


### CI/CD

* run the PostgreSQL tests against a Postgres service ([#170](https://github.com/hansjlachmann/openerp/issues/170)) ([86ad966](https://github.com/hansjlachmann/openerp/commit/86ad966482ba44041983b09f6be7ce544d70d5b2))

## [0.1.120](https://github.com/hansjlachmann/openerp/compare/v0.1.119...v0.1.120) (2026-10-29)


### Features

* balance check, posted tables, G/L Balance Check, Preview Posting ([#168](https://github.com/hansjlachmann/openerp/issues/168)) ([77644ec](https://github.com/hansjlachmann/openerp/commit/77644ecc0354f0db1ff364d73e8b65f3693a649e))

## [0.1.119](https://github.com/hansjlachmann/openerp/compare/v0.1.118...v0.1.119) (2026-10-29)


### Features

* My Settings — date and number format, short date entry, work date ([#166](https://github.com/hansjlachmann/openerp/issues/166)) ([98c0d1a](https://github.com/hansjlachmann/openerp/commit/98c0d1a5bfd852ca35b5db029c7a6232f67bca2b))

## [0.1.118](https://github.com/hansjlachmann/openerp/compare/v0.1.117...v0.1.118) (2026-10-29)


### Features

* Sales Invoice and Sales Credit Memo pages ([#164](https://github.com/hansjlachmann/openerp/issues/164)) ([77a7ba5](https://github.com/hansjlachmann/openerp/commit/77a7ba56b1a5c6a5b391bf0f63a3b7c87ce358e9))
* Sales Order — Make Order, quantities to ship and invoice, Order Confirmation ([#162](https://github.com/hansjlachmann/openerp/issues/162)) ([1be7020](https://github.com/hansjlachmann/openerp/commit/1be70206b9c150fa963f895e787a679c3b652f21))

## [0.1.117](https://github.com/hansjlachmann/openerp/compare/v0.1.116...v0.1.117) (2026-10-28)


### Features

* print a Sales Quote — Company Information, report engine, report 204 ([#160](https://github.com/hansjlachmann/openerp/issues/160)) ([7a075ab](https://github.com/hansjlachmann/openerp/commit/7a075ab5231a00465f8700a42786651da82876ab))

## [0.1.116](https://github.com/hansjlachmann/openerp/compare/v0.1.115...v0.1.116) (2026-10-28)


### Features

* Navigate — the entries of a document across the ledgers ([#157](https://github.com/hansjlachmann/openerp/issues/157)) ([396cf11](https://github.com/hansjlachmann/openerp/commit/396cf11a65aed2fc53d91d54a5d4b5da12c3dc49))
* Sales Quote — header and lines, prices, discounts, tax, invoice discount, statistics ([#159](https://github.com/hansjlachmann/openerp/issues/159)) ([2759dcc](https://github.com/hansjlachmann/openerp/commit/2759dcc0d3ee1387d7f7c02251817c9b9a41ace9))

## [0.1.115](https://github.com/hansjlachmann/openerp/compare/v0.1.114...v0.1.115) (2026-10-27)


### Features

* Job Queue and User Roles on their list pages, copy and paste ([#155](https://github.com/hansjlachmann/openerp/issues/155)) ([23f9e9b](https://github.com/hansjlachmann/openerp/commit/23f9e9b0b0eec2c8a5b161c824f24658a3443fd6))

## [0.1.114](https://github.com/hansjlachmann/openerp/compare/v0.1.113...v0.1.114) (2026-10-27)


### Features

* resources, item BOMs, cost adjustment and period grids ([#153](https://github.com/hansjlachmann/openerp/issues/153)) ([be5aa95](https://github.com/hansjlachmann/openerp/commit/be5aa95069823b352d35c02afa5cf1587000e044))

## [0.1.113](https://github.com/hansjlachmann/openerp/compare/v0.1.112...v0.1.113) (2026-10-25)


### Bug Fixes

* G/L account fields relate to posting accounts only ([#151](https://github.com/hansjlachmann/openerp/issues/151)) ([906e600](https://github.com/hansjlachmann/openerp/commit/906e6008f215ecac5190e18152d26fdfcce8612b))

## [0.1.112](https://github.com/hansjlachmann/openerp/compare/v0.1.111...v0.1.112) (2026-10-25)


### Features

* items — master data, prices, item journals and posting, picture ([#149](https://github.com/hansjlachmann/openerp/issues/149)) ([d24214d](https://github.com/hansjlachmann/openerp/commit/d24214d2775a9196ddf8ec334ce3022e526d71dd))

## [0.1.111](https://github.com/hansjlachmann/openerp/compare/v0.1.110...v0.1.111) (2026-10-24)


### Features

* execute permission for codeunits and reports; Job Queue runs as its user ([#146](https://github.com/hansjlachmann/openerp/issues/146)) ([33312b7](https://github.com/hansjlachmann/openerp/commit/33312b7384a9f99745ce805dfe286d633b7ccc94))

## [0.1.110](https://github.com/hansjlachmann/openerp/compare/v0.1.109...v0.1.110) (2026-10-24)


### Features

* main menu as the classic Main Menu — modules, Setup and Activities, menu search ([#144](https://github.com/hansjlachmann/openerp/issues/144)) ([59d731c](https://github.com/hansjlachmann/openerp/commit/59d731ce03c0af99cf9a146d4690cd1b046940bf))

## [0.1.109](https://github.com/hansjlachmann/openerp/compare/v0.1.108...v0.1.109) (2026-10-24)


### Features

* classic Customer, generated record behaviour, business logic as classic code ([#141](https://github.com/hansjlachmann/openerp/issues/141)) ([c5522f5](https://github.com/hansjlachmann/openerp/commit/c5522f5ade5fdf12ccce920cf42fcf00d0811c77))

## [0.1.108](https://github.com/hansjlachmann/openerp/compare/v0.1.107...v0.1.108) (2026-10-22)


### Features

* customer preparation — posting groups, codes, reminders, number series, sales setup ([58665c1](https://github.com/hansjlachmann/openerp/commit/58665c101fd509a94338cc50d1bf8c31fae6ad87))

## [0.1.107](https://github.com/hansjlachmann/openerp/compare/v0.1.106...v0.1.107) (2026-10-22)


### Features

* G/L entries with account totals, conditional table relations, more demo data ([a95fb02](https://github.com/hansjlachmann/openerp/commit/a95fb02cb656ff95e913df22af5b8dc50e20caab))
* **pages:** lookups open with Alt+Down only, New in the lookup ([5c94f80](https://github.com/hansjlachmann/openerp/commit/5c94f80fb6871708ba8ca00909403bfc801dc6f0))
* posting groups, tax areas and the chart of accounts (G/L Account) ([458553f](https://github.com/hansjlachmann/openerp/commit/458553fd97db6c67ab66dec818b27af0bd388a39))

## [0.1.106](https://github.com/hansjlachmann/openerp/compare/v0.1.105...v0.1.106) (2026-10-21)


### Features

* General Ledger Setup and Currency Exchange Rates (migration step 2) ([51e0e07](https://github.com/hansjlachmann/openerp/commit/51e0e072613ca07bf86473b12f39ee3d5eb3ec8b))


### Bug Fixes

* **pages:** setup card opens in view mode and Esc closes it ([a9fcc05](https://github.com/hansjlachmann/openerp/commit/a9fcc05f0887d97aa61439b42bf8d7a5c5e599a9))

## [0.1.105](https://github.com/hansjlachmann/openerp/compare/v0.1.104...v0.1.105) (2026-10-21)


### Features

* master data tables 3–14, field properties, Currency code and cards ([b7cebab](https://github.com/hansjlachmann/openerp/commit/b7cebab28508c058be58c5e3532cc12eaa2a6c62))


### Bug Fixes

* dates read back blank on PostgreSQL — a DATE column scanned into a string arrives ([b7cebab](https://github.com/hansjlachmann/openerp/commit/b7cebab28508c058be58c5e3532cc12eaa2a6c62))
* **pages:** modal cards for currencies and finance charge terms, one Esc, full screen key ([17aa0dc](https://github.com/hansjlachmann/openerp/commit/17aa0dc38499e88de7ae9cf1a2e134d32252c307))

## [0.1.104](https://github.com/hansjlachmann/openerp/compare/v0.1.103...v0.1.104) (2026-10-21)


### Features

* **pages:** arguments, sub-menus, card menus and functions, refreshing subpages ([0f96e6e](https://github.com/hansjlachmann/openerp/commit/0f96e6ea401053e069385a28e5b0903100ea6a23))

## [0.1.103](https://github.com/hansjlachmann/openerp/compare/v0.1.102...v0.1.103) (2026-10-21)


### Features

* **pages:** menu buttons and page functions; Esc closes every message box ([14918ea](https://github.com/hansjlachmann/openerp/commit/14918ea3364d3de521c086e21dfda42d50aa4635))
* **pages:** one action button everywhere, icons from the page YAML ([31deadf](https://github.com/hansjlachmann/openerp/commit/31deadfbce32de5c048e532346d4a531febc4e1d))

## [0.1.102](https://github.com/hansjlachmann/openerp/compare/v0.1.101...v0.1.102) (2026-10-20)


### Features

* **cards:** subpages — a list page inside a card, linked to its record ([37a58fe](https://github.com/hansjlachmann/openerp/commit/37a58feb9cd02d031684b06b505b4874a8a1817e))

## [0.1.101](https://github.com/hansjlachmann/openerp/compare/v0.1.100...v0.1.101) (2026-10-20)


### Features

* **cards:** scrolling with PgUp/PgDn, compact layout, collapsible sections ([dfc96df](https://github.com/hansjlachmann/openerp/commit/dfc96df8b13952f88f49aedc73f7b6fcf9cd88aa))
* **records:** Find next/previous, filter groups, selection filter, upper-limit flow filters ([2eead1b](https://github.com/hansjlachmann/openerp/commit/2eead1ba783b450cccec7c5f136aa0d6d0e963d6))
* **reports:** processing-only reports with request pages ([394fa8d](https://github.com/hansjlachmann/openerp/commit/394fa8d20ac1b5470655f3d6d9c0365680e55b25))
* **system:** CopyFilter, CompressArray, Hyperlink and the progress window ([5d29b13](https://github.com/hansjlachmann/openerp/commit/5d29b13d59ac20a263cb59864855fd4e0048ef57))
* **system:** temporary records, page variables with lookup mode, page triggers, closing dates, Consistent ([b9f4021](https://github.com/hansjlachmann/openerp/commit/b9f40213e0fbf39f752dc4d2ea34b3d20ed22feb))

## [0.1.100](https://github.com/hansjlachmann/openerp/compare/v0.1.99...v0.1.100) (2026-10-20)


### Features

* **system:** date formula fields and StrMenu ([7c1b8b6](https://github.com/hansjlachmann/openerp/commit/7c1b8b6ade4836632685c741db3ba008f1d024ab))


### Documentation

* TODO — StrMenu high priority, date formulas in Code fields, Find/FilterGroup plan ([734b291](https://github.com/hansjlachmann/openerp/commit/734b2919fb6786a8e98d419fa46d24081d520662))

## [0.1.99](https://github.com/hansjlachmann/openerp/compare/v0.1.98...v0.1.99) (2026-10-20)


### Features

* **system:** classic system functions and record methods for business logic ([4f6c088](https://github.com/hansjlachmann/openerp/commit/4f6c08823bbfcee70b3ade7a05e734dfe1dc3078))
* **system:** remaining classic system functions and record methods ([cd77be3](https://github.com/hansjlachmann/openerp/commit/cd77be3c3ba20f269c7be2149eeae7d89c8c5ec8))


### Documentation

* TODO — production on 0.1.97, release step failing ([08bc51b](https://github.com/hansjlachmann/openerp/commit/08bc51b19e6c5df097a87281544041ffb59c3c16))
* TODO — release token, renew before it expires ([4aca61b](https://github.com/hansjlachmann/openerp/commit/4aca61bc1fa5f67944d61db1871116e8df8e04fe))


### CI/CD

* give release-please the issues permission it needs to label releases ([6ca667d](https://github.com/hansjlachmann/openerp/commit/6ca667dff30ec555587b256cd02861e069d7f58c))
* release-please uses a repository token to create releases ([1ec321e](https://github.com/hansjlachmann/openerp/commit/1ec321e84db2d758b9b8cf28ce07decb21205ab5))
* run on Ubuntu 24.04 (Playwright has no browsers for 26.04 yet) ([7f6d2f5](https://github.com/hansjlachmann/openerp/commit/7f6d2f5201422810db995479f64fff291178bae2))

## [0.1.98](https://github.com/hansjlachmann/openerp/compare/v0.1.97...v0.1.98) (2026-10-16)


### Bug Fixes

* **preferences:** save and read user preferences ([bbcee13](https://github.com/hansjlachmann/openerp/commit/bbcee13d3dec01e2b0c22cbbce79b5fb3c055b1a))


### Documentation

* CLAUDE.md wording fixes ([f434315](https://github.com/hansjlachmann/openerp/commit/f43431526cddf428029778a40f50b87075e9238f))
* TODO — production upgrade to 0.1.97, neutral wording done, request validation ([b60deee](https://github.com/hansjlachmann/openerp/commit/b60deeef4acc811f10f73a5101b6bb9795ae7c58))

## [0.1.97](https://github.com/hansjlachmann/openerp/compare/v0.1.96...v0.1.97) (2026-10-13)


### Features

* **apps:** application.yaml, classic object numbers, installed apps, field number guard ([8102986](https://github.com/hansjlachmann/openerp/commit/8102986cc3c0f8419a2410bff9607dfcd223fc3c))
* **tables:** field numbers in table definitions ([6ccc6a9](https://github.com/hansjlachmann/openerp/commit/6ccc6a97390d086502e591d66cae015d5535e0fe))


### Code Refactoring

* neutral wording, built-in themes renamed to Classic Light/Dark ([63d4401](https://github.com/hansjlachmann/openerp/commit/63d4401bfdc330f65fab3730a170a050acdabfc5))
* separate generated table code from the business logic ([1a98c98](https://github.com/hansjlachmann/openerp/commit/1a98c9838ac70fcaf8f222dff93a7e152632f4d0))


### Documentation

* TODO — codeunit permission check in the first priority list ([1ca2c6d](https://github.com/hansjlachmann/openerp/commit/1ca2c6d247297f72a23b5073a26a42ed6246da0b))
* TODO — codeunit permission check is medium priority, to be specified ([f938c6e](https://github.com/hansjlachmann/openerp/commit/f938c6e3d0abf70e30aed54a0a5f7a2a7e6a9e3d))
* TODO — decisions for the system functions, field numbers (high priority) ([1f8825b](https://github.com/hansjlachmann/openerp/commit/1f8825b6009d4af43eea9cfbd343148b580f4241))
* TODO — remaining system functions are high priority ([473c4f5](https://github.com/hansjlachmann/openerp/commit/473c4f53a1b3c70834e156c4921c05ca4f2e3ff4))
* TODO — second set of system functions, remaining ones as a later item ([e8a9431](https://github.com/hansjlachmann/openerp/commit/e8a9431e87c2fbd0a640b43e48bc3e1c49291bd0))
* TODO — specification of system functions in Go (first priority) ([d0d61ec](https://github.com/hansjlachmann/openerp/commit/d0d61ec4d3ab93f137f84c09cd59a2dc252c2f8e))

## [0.1.96](https://github.com/hansjlachmann/openerp/compare/v0.1.95...v0.1.96) (2026-10-12)


### Features

* **home:** Welcome to EasyopenERP, tip to learn the keyboard shortcuts ([5acaf14](https://github.com/hansjlachmann/openerp/commit/5acaf14cb318609a93b68e516685907794ae1a59))
* **theme:** color themes per user, chosen in an Appearance dialog ([f480212](https://github.com/hansjlachmann/openerp/commit/f480212de4d20a553a50f98ad40f37dcf4cd1349))
* **theme:** every user can install, update and remove their own themes ([594486a](https://github.com/hansjlachmann/openerp/commit/594486a15e9887c8822f653a4690912bb6426140))
* **theme:** font, font size and list density per user ([b174eaf](https://github.com/hansjlachmann/openerp/commit/b174eaf15fcf5b1abd7b0bca57f525a4ac789768))
* **theme:** install Omarchy themes from GitHub ([1b8e68a](https://github.com/hansjlachmann/openerp/commit/1b8e68a4b5f6272e1dea03a1e5c74622d794ad9b))
* **theme:** installed themes are per user ([ab60a01](https://github.com/hansjlachmann/openerp/commit/ab60a0147e4f9a77548e7a6b2c3cbee09862a298))
* **theme:** link to the Omarchy theme gallery in the install box ([9966bb0](https://github.com/hansjlachmann/openerp/commit/9966bb0674e2aab4850a17f1467e29d5cc26453e))
* **theme:** wallpapers of installed themes ([4c17845](https://github.com/hansjlachmann/openerp/commit/4c178453ffa86f966540141f1b17d84b433b9b85))


### Code Refactoring

* **theme:** colors as CSS variables (no visual change) ([7b96820](https://github.com/hansjlachmann/openerp/commit/7b968204b48cecc60d445c6ebbf1fd814bf11fdf))


### Documentation

* TODO — list and card adjustments of 2026-10-12, Ctrl+F7 note for Phase 3 ([ade08ed](https://github.com/hansjlachmann/openerp/commit/ade08ed4d3645b1f3485083c631becfb6ed852a3))
* TODO — themes and appearance done, codeunit permission gap (high priority) ([83f58e8](https://github.com/hansjlachmann/openerp/commit/83f58e853b763c4002898b5fee105b5644b4d056))

## [0.1.95](https://github.com/hansjlachmann/openerp/compare/v0.1.94...v0.1.95) (2026-10-12)


### Features

* **list:** copy/paste of records, drag-select and keyboard filtering with Ctrl+F7 ([da07e34](https://github.com/hansjlachmann/openerp/commit/da07e34399c131a91180b3d0fd5d7dfc6aaaf68e))


### Bug Fixes

* "Error Loading Page" explains an expired session and offers a new login ([228c15b](https://github.com/hansjlachmann/openerp/commit/228c15b97d83430f6aac2c72ff82a59f0d317771))


### Documentation

* business logic rule (backend only) and a Business Logic section in the TODO ([48739dc](https://github.com/hansjlachmann/openerp/commit/48739dc69dab8cd725af6c5a8d74c42698fa17cc))

## [0.1.94](https://github.com/hansjlachmann/openerp/compare/v0.1.93...v0.1.94) (2026-10-11)


### Documentation

* TODO and CLAUDE.md — actions on a selection are not wanted ([ef6521d](https://github.com/hansjlachmann/openerp/commit/ef6521d407ba96947b879b55f193d43dba293de5))

## [0.1.93](https://github.com/hansjlachmann/openerp/compare/v0.1.92...v0.1.93) (2026-10-11)


### Bug Fixes

* one click selects a list row, no Refresh action, no loading flash ([ba91fd5](https://github.com/hansjlachmann/openerp/commit/ba91fd5cb2c492a41991257746612b440c9a526c))


### Performance

* open pages inside the app instead of reloading it ([403c5e0](https://github.com/hansjlachmann/openerp/commit/403c5e059fb5a51e7329632d724a2cefbbf3d7cd))

## [0.1.92](https://github.com/hansjlachmann/openerp/compare/v0.1.91...v0.1.92) (2026-10-11)


### Features

* column header menus, sorting by caption and on FlowFields ([6e07893](https://github.com/hansjlachmann/openerp/commit/6e07893182baf0595ba64015b08fd2f8742799ce))
* select several rows on lists and delete them together ([ecad83a](https://github.com/hansjlachmann/openerp/commit/ecad83a1d7e3afdfee2e1fe3150aa4016f2b77a0))


### Bug Fixes

* login asks for the company last and names a missing company access ([1511548](https://github.com/hansjlachmann/openerp/commit/1511548c4c7c0d4d919bcff03411c4fcde2cc8f7))


### Documentation

* TODO — old backend images deleted from ghcr.io ([f55e37a](https://github.com/hansjlachmann/openerp/commit/f55e37a3ee4760a5da619ff70f9bba03dac47063))
* TODO — production upgraded to 0.1.91 ([42b7225](https://github.com/hansjlachmann/openerp/commit/42b722524f8f3820e4b5b3b57326270fc01500a0))

## [0.1.91](https://github.com/hansjlachmann/openerp/compare/v0.1.90...v0.1.91) (2026-10-11)


### Bug Fixes

* frontend image build no longer hangs on arm64 ([b40868b](https://github.com/hansjlachmann/openerp/commit/b40868badb5c0545d3a2d694168bd22af0f6aa52))

## [0.1.90](https://github.com/hansjlachmann/openerp/compare/v0.1.89...v0.1.90) (2026-10-09)


### Features

* filter lists on FlowFields; translated buttons, sections and option values ([317cbb7](https://github.com/hansjlachmann/openerp/commit/317cbb7388abd4e43d88f09b327d26083252348a))

## [0.1.89](https://github.com/hansjlachmann/openerp/compare/v0.1.88...v0.1.89) (2026-10-09)


### Features

* encrypt secret fields at rest (encrypted: true), SMTP password first ([c0915c8](https://github.com/hansjlachmann/openerp/commit/c0915c8ad0b2ec7c43bec501a9327f97456975b1))


### Documentation

* TODO — GitHub Support purge requested ([711e1a8](https://github.com/hansjlachmann/openerp/commit/711e1a867e9b507d1843af4ed133161499091d9d))

## [0.1.88](https://github.com/hansjlachmann/openerp/compare/v0.1.87...v0.1.88) (2026-10-09)


### Documentation

* CHANGELOG links after the history rewrite, TODO follow-ups ([58d6d2b](https://github.com/hansjlachmann/openerp/commit/58d6d2b2d9f2ad90a2887536f8e6fd93d48fa000))
* public repository rule — no confidential or installation details ([833c079](https://github.com/hansjlachmann/openerp/commit/833c07903f733850b8d404985ce25ce464f12245))
* TODO — production upgrade straight to 0.1.87 ([e03e4c1](https://github.com/hansjlachmann/openerp/commit/e03e4c1c0998529c2e76d4b30cc2751b6150c239))

## [0.1.87](https://github.com/hansjlachmann/openerp/compare/v0.1.86...v0.1.87) (2026-10-08)


### Features

* Job Queue e-mail notifications to several addresses ([4b1ec44](https://github.com/hansjlachmann/openerp/commit/4b1ec441b9b4329a23eddc80fa4d93a33ebb143d))

## [0.1.86](https://github.com/hansjlachmann/openerp/compare/v0.1.85...v0.1.86) (2026-10-08)


### Bug Fixes

* READER role can read Countries/Regions and SMTP Setup ([2556655](https://github.com/hansjlachmann/openerp/commit/2556655045fbdc9ad0963bf4132f4f1cebe47ad8))
* remove the intro line from the Keyboard Shortcuts help ([5149202](https://github.com/hansjlachmann/openerp/commit/51492023a5947692787bec5e5f6118ed535a2168))
* translated validation messages with the field's caption ([5266f89](https://github.com/hansjlachmann/openerp/commit/5266f894378fba7b78de3321e699a8009d2a3e9e))


### Documentation

* TODO — Job Queue e-mail notification to one or more addresses ([61516c3](https://github.com/hansjlachmann/openerp/commit/61516c3ac6633669c2956a2c6cd3d4662e7c6022))
* TODO — primary key cell keying was already fixed ([181e520](https://github.com/hansjlachmann/openerp/commit/181e520975b7ce386a8f5fd7e176795320981a98))
* TODO — production upgrade to 0.1.85 planned, steps and checks ([7a700c0](https://github.com/hansjlachmann/openerp/commit/7a700c0d2e19c581e2f3aef20eabd97d4b25e9e6))


### CI/CD

* fail the Docker build after 30 minutes instead of hanging ([0bbc90e](https://github.com/hansjlachmann/openerp/commit/0bbc90e9ddc1095c8162d27d76c00c01a01eb1c8))

## [0.1.85](https://github.com/hansjlachmann/openerp/compare/v0.1.84...v0.1.85) (2026-10-08)


### Bug Fixes

* card navigation follows the list's filters, search and sort ([555c8ba](https://github.com/hansjlachmann/openerp/commit/555c8ba0f0eea8b915c6e14dad7c8eebc5dbe8e3))
* case-insensitive list search for æ, ø, å on SQLite ([5c29579](https://github.com/hansjlachmann/openerp/commit/5c29579d1167e110b5b59f18818b73931a7f40b9))
* list cells respect editable: false and FlowField columns ([5fa7d15](https://github.com/hansjlachmann/openerp/commit/5fa7d15cd420a80407798fe1a9da26f60649e038))
* menu bar shows the new company name right after renaming it ([9093ef6](https://github.com/hansjlachmann/openerp/commit/9093ef61995dfdeafb179479a3e01a1c46780384))
* read the NAV proxy URL from NAV_PROXY_URL instead of a built-in address ([ce4bd6f](https://github.com/hansjlachmann/openerp/commit/ce4bd6f77f9e094233b8e313e0851618a4c6624c))
* rename a record on its card ([e474117](https://github.com/hansjlachmann/openerp/commit/e47411740b3317ea8cf6191a1ecaad4f90c83139))
* say that the key exists when a record is renamed to an existing key ([a99b226](https://github.com/hansjlachmann/openerp/commit/a99b2268e15daa371acc4d5582752d94556cd7bf))
* uppercase Code fields in list cells before saving ([891706e](https://github.com/hansjlachmann/openerp/commit/891706e792335d8b25530a3d43267b251756d580))


### Performance

* bulk-insert demo ledger entries and keep SIFT totals free of dead rows ([64905c7](https://github.com/hansjlachmann/openerp/commit/64905c77f4dd7d0720eb78a4156b0eb648ed80d8))


### Documentation

* field type names the page metadata sends ([dc7edc4](https://github.com/hansjlachmann/openerp/commit/dc7edc4a0ca9d18472bf4251685fc6b08af769e8))
* TODO — demo jwt-secret done ([9da2b63](https://github.com/hansjlachmann/openerp/commit/9da2b63638f50a8cf0034ff0577d52bdd6aba6fa))
* TODO — NAV_PROXY_URL before the next production upgrade, history cleanup ([e3b98fa](https://github.com/hansjlachmann/openerp/commit/e3b98fa8678078c2d5bc3c13abbb4078c8dec35d))
* TODO — production NAV_PROXY_URL set, steps for the upgrade to 0.1.85 ([ea4073e](https://github.com/hansjlachmann/openerp/commit/ea4073e9a680973d28c6b68c163e474dd205b5e7))
* TODO — production upgraded to 0.1.84, follow-ups ([05df2a2](https://github.com/hansjlachmann/openerp/commit/05df2a2a0a6fcf5d96612a165e35cc798fba315a))
* TODO first priority — demo jwt-secret, production upgrade to 0.1.84 ([9add3ee](https://github.com/hansjlachmann/openerp/commit/9add3eebbcb7d72a2069e0e2b402914227356c1b))

## [0.1.84](https://github.com/hansjlachmann/openerp/compare/v0.1.83...v0.1.84) (2026-10-07)


### Bug Fixes

* harden the production setup ([36533ed](https://github.com/hansjlachmann/openerp/commit/36533ed75f38643d050809d40e98bcc020da77e9))
* remove deleted records from editable lists at once ([7d3f9c5](https://github.com/hansjlachmann/openerp/commit/7d3f9c5bcc77a2e861895312ec833beb21af99a4))

## [0.1.83](https://github.com/hansjlachmann/openerp/compare/v0.1.82...v0.1.83) (2026-10-07)


### Documentation

* TODO for the New User autofill fix and the hanging Docker build ([c4790ee](https://github.com/hansjlachmann/openerp/commit/c4790ee537ab489c297faab7c7a9bea7e5991def))

## [0.1.82](https://github.com/hansjlachmann/openerp/compare/v0.1.81...v0.1.82) (2026-10-07)


### Bug Fixes

* insert a new card record only once its key is filled in ([121a6b6](https://github.com/hansjlachmann/openerp/commit/121a6b64f8497c39bf1d77e2efb650366d685f4e))

## [0.1.81](https://github.com/hansjlachmann/openerp/compare/v0.1.80...v0.1.81) (2026-10-07)


### Features

* rename a company with its data (Rename) ([5a462e4](https://github.com/hansjlachmann/openerp/commit/5a462e4d330abea6cfce3fffe3bb5e9b80aa7621))


### Bug Fixes

* mask write-only secrets like the SMTP password (Masked) ([ca7f923](https://github.com/hansjlachmann/openerp/commit/ca7f92310883e40e199b4beb2a3a51f63e1f7c98))
* never expose password hashes through the table API ([b0bb483](https://github.com/hansjlachmann/openerp/commit/b0bb4839114968332951826c6dd2d23bcf59833b))


### Code Refactoring

* remove Init without database type ([bc802ea](https://github.com/hansjlachmann/openerp/commit/bc802ea86c2d6407433c54697ded26a7c98b3e72))

## [0.1.80](https://github.com/hansjlachmann/openerp/compare/v0.1.79...v0.1.80) (2026-10-06)


### Performance

* remove the Customer Ledger Entry date SIFT key ([3cba1a5](https://github.com/hansjlachmann/openerp/commit/3cba1a5139a02b84418fe69003648711a938b088))

## [0.1.79](https://github.com/hansjlachmann/openerp/compare/v0.1.78...v0.1.79) (2026-10-06)


### Features

* FlowFilter Date Filter, Rename cascade, Verify SIFT codeunit, HEAVY demo data ([732b525](https://github.com/hansjlachmann/openerp/commit/732b52563aa7ad9d544428ea8c0a559f581cf010))

## [0.1.78](https://github.com/hansjlachmann/openerp/compare/v0.1.77...v0.1.78) (2026-10-06)


### Features

* on-demand dropdowns, Customer Ledger Entries page, drilldown return ([8aed7d4](https://github.com/hansjlachmann/openerp/commit/8aed7d4975a9299156276b845e7ec1104e2627e7))
* SIFT totals tables for FlowFields (SumIndexFields) ([2d06d9e](https://github.com/hansjlachmann/openerp/commit/2d06d9ee1e753ff157f077e45475ea41b0ae3b4c))


### Bug Fixes

* keep focus and save only real changes when editing list cells ([5007326](https://github.com/hansjlachmann/openerp/commit/5007326daeb3f9297c3352271547cc0b67842249))

## [0.1.77](https://github.com/hansjlachmann/openerp/compare/v0.1.76...v0.1.77) (2026-10-06)


### Performance

* calculate list FlowFields with one grouped query per field ([2907efc](https://github.com/hansjlachmann/openerp/commit/2907efcfdc886dadf1bfd8b57dd3ba12b678fcdd))
* windowed list loading with server-side search and sort ([a28a4b7](https://github.com/hansjlachmann/openerp/commit/a28a4b7a333e746385cf496a7e4d2efff59744f3))

## [0.1.76](https://github.com/hansjlachmann/openerp/compare/v0.1.75...v0.1.76) (2026-10-05)


### Documentation

* TODO high-priority list performance with large data ([211f16f](https://github.com/hansjlachmann/openerp/commit/211f16f380823da992f1f2df8f0bf1ae6dc226e1))

## [0.1.75](https://github.com/hansjlachmann/openerp/compare/v0.1.74...v0.1.75) (2026-10-05)


### Features

* demo data via Job Queue, Country/Region table, company display name ([0d7bfc0](https://github.com/hansjlachmann/openerp/commit/0d7bfc01ad846a89773f3bd10a789519a494ce19))

## [0.1.74](https://github.com/hansjlachmann/openerp/compare/v0.1.73...v0.1.74) (2026-10-05)


### Documentation

* TODO remove intro text from keyboard shortcuts help ([5e942ca](https://github.com/hansjlachmann/openerp/commit/5e942ca0807256b7e1540570f5f1ba02d090ee36))

## [0.1.73](https://github.com/hansjlachmann/openerp/compare/v0.1.72...v0.1.73) (2026-10-05)


### Features

* Ctrl+O switch company, Enter to next field on cards, keyboard shortcuts help ([ad43454](https://github.com/hansjlachmann/openerp/commit/ad434543d30b7126ee2010a84cf5250f02ac73c3))

## [0.1.72](https://github.com/hansjlachmann/openerp/compare/v0.1.71...v0.1.72) (2026-10-05)


### Bug Fixes

* F8/F2 in list cells place the cursor at the end instead of selecting all ([18fa8ea](https://github.com/hansjlachmann/openerp/commit/18fa8ea27f2298e6c88589a51348236d48a7f9d9))
* list scrolling hid rows behind the sticky header; highlight selected cell text ([63a8a39](https://github.com/hansjlachmann/openerp/commit/63a8a39b9b68c27be4cdc90680ddf20d933f329c))

## [0.1.71](https://github.com/hansjlachmann/openerp/compare/v0.1.70...v0.1.71) (2026-10-05)


### Bug Fixes

* dialog buttons showed raw translation keys (BTN_CANCEL) ([3e947db](https://github.com/hansjlachmann/openerp/commit/3e947db5fe7c869fd74d2b59737b030dd4f211ef))
* list editing with search/sort, lookup Tab/Enter, PageUp/Down, DelayedInsert ([5bebc51](https://github.com/hansjlachmann/openerp/commit/5bebc512af1ca29df54a210725c9cdb7ac7f1d7f))

## [0.1.70](https://github.com/hansjlachmann/openerp/compare/v0.1.69...v0.1.70) (2026-10-05)


### Bug Fixes

* list Edit/Delete acted on the wrong record when searched or sorted ([9cdbe0b](https://github.com/hansjlachmann/openerp/commit/9cdbe0b440d46dacd20482e7814f3c1999cb8a91))
* show trigger error messages and fix user preferences cascade ([02317be](https://github.com/hansjlachmann/openerp/commit/02317be02313397add716319832cfdbcf332cea5))


### Documentation

* add deployment and operations follow-ups to TODO ([a2e31d4](https://github.com/hansjlachmann/openerp/commit/a2e31d47c4a430562f6471d0d3cb2ed6a15e8e16))

## [0.1.69](https://github.com/hansjlachmann/openerp/compare/v0.1.68...v0.1.69) (2026-10-03)


### Features

* arrow-key navigation on the main menu; align Customer List shortcuts ([d2c2d41](https://github.com/hansjlachmann/openerp/commit/d2c2d41a3c22471f5d360c07a2b11181934ac79b))
* record entry on editable list pages ([1d66335](https://github.com/hansjlachmann/openerp/commit/1d66335717e3caa467672e2c1f5f153ec4dba5f4))
* classic ERP colors, light list header, wider scrollbars, Alt+N for New ([bb121b7](https://github.com/hansjlachmann/openerp/commit/bb121b7d7a9eeed6952a474f27e65f9107d2cd7d))


### Bug Fixes

* prevent SQL injection through filter and sort field names ([8c1412f](https://github.com/hansjlachmann/openerp/commit/8c1412f1d1a38bc18d33e84541e35631d000acb5))
* subtler list cell selection frame that stays inside its cell ([d52cc89](https://github.com/hansjlachmann/openerp/commit/d52cc89b5a1c9261d91c17773fef05da4914fea7))


### Documentation

* record follow-ups from record entry and SQL injection work ([c518f94](https://github.com/hansjlachmann/openerp/commit/c518f94a4207f12403628b6e46ee7a993dcc1466))

## [0.1.68](https://github.com/hansjlachmann/openerp/compare/v0.1.67...v0.1.68) (2026-07-20)


### Documentation

* spec list page parity and editable-list record entry behavior ([e56b047](https://github.com/hansjlachmann/openerp/commit/e56b047a6f3fb0f777df540b519b53279556a315))

## [0.1.67](https://github.com/hansjlachmann/openerp/compare/v0.1.66...v0.1.67) (2026-07-18)


### Features

* add opt-in server-side pagination to list endpoint ([9bc216e](https://github.com/hansjlachmann/openerp/commit/9bc216e834417054a91dbc714fb61839370107d5))
* automatic Job Queue scheduler with email notifications and DB-backed SMTP setup ([a17f2fc](https://github.com/hansjlachmann/openerp/commit/a17f2fc2efd75841866cd9a8c5bef8a67a7e3f7d))


### Documentation

* mark pagination done; add Job Queue scheduler + email spec ([d33b23c](https://github.com/hansjlachmann/openerp/commit/d33b23c0e617c54aa90a85d819655fe65c255c8b))

## [0.1.66](https://github.com/hansjlachmann/openerp/compare/v0.1.65...v0.1.66) (2026-07-15)


### Features

* add IP-based rate limiting to the API ([e09ab20](https://github.com/hansjlachmann/openerp/commit/e09ab20796eb38250abbfea4abc645dae307cab2))
* add SQLite-safe RecreateTable migration helper ([29ddc62](https://github.com/hansjlachmann/openerp/commit/29ddc62206f891862b27aa1740b64d384beeaef1))


### Bug Fixes

* cascade-delete user preferences on user delete ([8297baa](https://github.com/hansjlachmann/openerp/commit/8297baaef67559911ec7003efb67336cbd5a8c17))
* create Menu table in migration 003 before seeding ([73f68d7](https://github.com/hansjlachmann/openerp/commit/73f68d749b0fa573b3869c2015b7ee3d3f621d20))


### Code Refactoring

* remove unused Repository abstraction ([4eba561](https://github.com/hansjlachmann/openerp/commit/4eba5612b3214783f313172fc92afadceef17494))


### Documentation

* add consolidated TODO.md backlog and reconcile stale sub-READMEs ([76c115a](https://github.com/hansjlachmann/openerp/commit/76c115a55bb10614749782c1cfc130d57e922816))

## [0.1.65](https://github.com/hansjlachmann/openerp/compare/v0.1.64...v0.1.65) (2026-03-26)


### Bug Fixes

* permission check case mismatch after uppercase migration ([ebee053](https://github.com/hansjlachmann/openerp/commit/ebee053b2bd2b325d85298e1ab4ab4a6a672f723))

## [0.1.64](https://github.com/hansjlachmann/openerp/compare/v0.1.63...v0.1.64) (2026-03-26)


### Bug Fixes

* boolean checkbox save in navigation mode and Permission key casing ([c62770b](https://github.com/hansjlachmann/openerp/commit/c62770bb524c63460f40ee0fa3fb4cabaf01d300))
* boolean checkboxes not clickable on editable list pages ([b589e8c](https://github.com/hansjlachmann/openerp/commit/b589e8c121e6e991fbfcd25a3a5b0880f8fc16a6))

## [0.1.63](https://github.com/hansjlachmann/openerp/compare/v0.1.62...v0.1.63) (2026-03-24)


### Bug Fixes

* Job Queue entry error leak and incorrect end time ([70fd4ad](https://github.com/hansjlachmann/openerp/commit/70fd4ad56a40ff311acf6d9505330c465ae668d9))

## [0.1.62](https://github.com/hansjlachmann/openerp/compare/v0.1.61...v0.1.62) (2026-03-19)


### Features

* dynamic dialog fields from Job Queue JSON parameter ([61b63b9](https://github.com/hansjlachmann/openerp/commit/61b63b92c081f36c30421b911da08ef71f6f606e))
* replace native select with OptionDropdown for Option fields ([0b88562](https://github.com/hansjlachmann/openerp/commit/0b885627fd79f22e8e553146c397f7dca7a96cac))


### Bug Fixes

* OptionDropdown Enter key should move to next row when closed ([e357bff](https://github.com/hansjlachmann/openerp/commit/e357bff731a5c9b3c01197bdd4b59d3b57688e97))
* OptionDropdown not opening due to missing tabindex default ([d619a56](https://github.com/hansjlachmann/openerp/commit/d619a56271fa91fcd944056f80225c5aa45a2105))


### Documentation

* clarify OptionDropdown Enter key behavior in CLAUDE.md ([30f189c](https://github.com/hansjlachmann/openerp/commit/30f189c6f3d15523d0f25dad345ce92a1d168fc1))

## [0.1.61](https://github.com/hansjlachmann/openerp/compare/v0.1.60...v0.1.61) (2026-03-19)


### Features

* add Ctrl+C/V copy-paste support in list page cell-selected mode ([3c20c93](https://github.com/hansjlachmann/openerp/commit/3c20c9332fca300b2a5e81334ae048ac04d2a944))

## [0.1.60](https://github.com/hansjlachmann/openerp/compare/v0.1.59...v0.1.60) (2026-03-18)


### Bug Fixes

* Job Queue entry error leak, missing translation, and post-job refresh ([133e968](https://github.com/hansjlachmann/openerp/commit/133e9689bf068ad4b5b544a7cf781687e0676d9d))

## [0.1.59](https://github.com/hansjlachmann/openerp/compare/v0.1.58...v0.1.59) (2026-03-18)


### Features

* add Job Queue error handling, flow field, and drilldown navigation ([4a5ba25](https://github.com/hansjlachmann/openerp/commit/4a5ba25b1e2e71e94cc300a040a8a21c5b65d862))

## [0.1.58](https://github.com/hansjlachmann/openerp/compare/v0.1.57...v0.1.58) (2026-03-18)


### Bug Fixes

* disable nginx proxy buffering for SSE streaming ([30a8f00](https://github.com/hansjlachmann/openerp/commit/30a8f00bd9452be51504107ac8925bcb539bb8e2))

## [0.1.57](https://github.com/hansjlachmann/openerp/compare/v0.1.56...v0.1.57) (2026-03-18)


### Features

* replace global session with per-request JWT cookie sessions ([b649753](https://github.com/hansjlachmann/openerp/commit/b649753723dffa84d1403b00a7f224f6f8004b4f))

## [0.1.56](https://github.com/hansjlachmann/openerp/compare/v0.1.55...v0.1.56) (2026-03-12)


### Features

* locale-aware date/datetime formatting across all frontend pages ([beea75d](https://github.com/hansjlachmann/openerp/commit/beea75da5668969d64872d0830c16d8464c4e4fd))

## [0.1.55](https://github.com/hansjlachmann/openerp/compare/v0.1.54...v0.1.55) (2026-03-12)


### Bug Fixes

* URL-decode record ID in parseRecordKey for composite keys ([56f642f](https://github.com/hansjlachmann/openerp/commit/56f642f80bf40388a888c3a38939138651c9f824))

## [0.1.54](https://github.com/hansjlachmann/openerp/compare/v0.1.53...v0.1.54) (2026-03-12)


### Bug Fixes

* detect job disappearing after POST failure in NavReportRunner ([abc7745](https://github.com/hansjlachmann/openerp/commit/abc77455a34c81795cd96d8935f3fbd7d004b922))
* guard Object.assign against null savedRecord in ListPage ([43a5262](https://github.com/hansjlachmann/openerp/commit/43a526210aab8dddf3b22748642be4b27fd109ff))
* make parameter keys case-insensitive in NavReportRunner ([9ef1afe](https://github.com/hansjlachmann/openerp/commit/9ef1afe2720f737f87d215dbdabfc3bb44eb5ff2))

## [0.1.53](https://github.com/hansjlachmann/openerp/compare/v0.1.52...v0.1.53) (2026-03-12)


### Bug Fixes

* **ci:** remove duplicate .env write step that caused unstaged changes ([f3860a3](https://github.com/hansjlachmann/openerp/commit/f3860a32821fe7961daa6a6b548030d24fa39952))

## [0.1.52](https://github.com/hansjlachmann/openerp/compare/v0.1.51...v0.1.52) (2026-03-12)


### Bug Fixes

* **ci:** pull before writing .env to avoid rebase conflicts ([4cbb252](https://github.com/hansjlachmann/openerp/commit/4cbb252d9997bc51b41200b8d09fc87851ab6879))

## [0.1.51](https://github.com/hansjlachmann/openerp/compare/v0.1.50...v0.1.51) (2026-03-11)


### Features

* support optional PDF output in NavReportRunner (codeunit 50022) ([bccd228](https://github.com/hansjlachmann/openerp/commit/bccd228a2a1e45d917c446edb3fc25e0d1dc360d))

## [0.1.50](https://github.com/hansjlachmann/openerp/compare/v0.1.49...v0.1.50) (2026-03-11)


### Bug Fixes

* allow click-to-select in LookupDropdown ([c6ce4a8](https://github.com/hansjlachmann/openerp/commit/c6ce4a8cfb5fa72ca83e10522a3dd1a1a352990f))
* change lookup dropdown shortcut from Ctrl+ArrowDown to Alt+ArrowDown ([10a8fd5](https://github.com/hansjlachmann/openerp/commit/10a8fd59772e0adcb732e032a1c3f80ccb52a770))
* improve lookup field interaction in list page cell model ([c57cacd](https://github.com/hansjlachmann/openerp/commit/c57cacda25e0201cf7e0bca012e5835c9bc75fc9))
* prevent auto-escalation to cell-editing on new rows and lookup fields ([ce306d1](https://github.com/hansjlachmann/openerp/commit/ce306d1bfb6e54e08216d4fbacd501d806aeba30))


### Documentation

* update CLAUDE.md with lookup field interaction patterns ([e7243b8](https://github.com/hansjlachmann/openerp/commit/e7243b8f7b53a3c3888cc93e6a1a9a9f7956deba))

## [0.1.49](https://github.com/hansjlachmann/openerp/compare/v0.1.48...v0.1.49) (2026-03-11)


### Documentation

* document forceInsert bypass for delayed insert in CLAUDE.md ([ee1985d](https://github.com/hansjlachmann/openerp/commit/ee1985d13facf685880cedffb6eee458b04e1df0))

## [0.1.48](https://github.com/hansjlachmann/openerp/compare/v0.1.47...v0.1.48) (2026-03-11)


### Bug Fixes

* delayed insert not firing when pressing Enter on new record ([2c24f1f](https://github.com/hansjlachmann/openerp/commit/2c24f1f1704f086997c26cf1400a53b91cc9475b))

## [0.1.47](https://github.com/hansjlachmann/openerp/compare/v0.1.46...v0.1.47) (2026-03-11)


### Bug Fixes

* add Ctrl+E shortcut in navigation mode and remove outdated Code field gap note ([a3fca62](https://github.com/hansjlachmann/openerp/commit/a3fca62a018e23638fc31a76b148737f0ebff43e))

## [0.1.46](https://github.com/hansjlachmann/openerp/compare/v0.1.45...v0.1.46) (2026-03-11)


### Features

* add F5 (refresh) and Ctrl+D (delete) keyboard shortcuts in navigation mode ([ab49d93](https://github.com/hansjlachmann/openerp/commit/ab49d931240ae6d1802dc6b8d85c9b938002c786))
* add i18n support for codeunit dialog titles and messages ([997798d](https://github.com/hansjlachmann/openerp/commit/997798db4525f7a38d4db951d011ce822ebb027a))
* implement 3-state spreadsheet cell model in ListPage ([78d28d9](https://github.com/hansjlachmann/openerp/commit/78d28d939eef776f4368d5d498ff911339b9ed21))

## [0.1.45](https://github.com/hansjlachmann/openerp/compare/v0.1.44...v0.1.45) (2026-02-19)


### Features

* add RequestInput dialog for codeunits to collect user input mid-execution ([b63a685](https://github.com/hansjlachmann/openerp/commit/b63a685648b4e154148c07041409cf09b530fa5c))

## [0.1.44](https://github.com/hansjlachmann/openerp/compare/v0.1.43...v0.1.44) (2026-02-13)


### Features

* add HTTPS support with self-signed certificates for internal network ([2cc14d7](https://github.com/hansjlachmann/openerp/commit/2cc14d73a2901d2413e0e894e523dd2a6ffab638))
* log job queue entries for failed report executions ([ae971ef](https://github.com/hansjlachmann/openerp/commit/ae971efc1fe1dab3670da5433211e7341d112679))


### Bug Fixes

* add $state() to confirmResponseCallback to resolve Svelte warning ([6b77f52](https://github.com/hansjlachmann/openerp/commit/6b77f52966be084878215736f0e1d66ff6172c16))
* resolve empty branch lint error in page registry LoadMenu call ([de66419](https://github.com/hansjlachmann/openerp/commit/de6641995169795793105145ab101e32f65f809c))
* show animated "Saving PDF..." while waiting for startjob response ([8e9e6f1](https://github.com/hansjlachmann/openerp/commit/8e9e6f11f90beeffe5388a0d78b8a5f0b91b611d))
* suppress warning for missing default menu.yaml at startup ([671bda4](https://github.com/hansjlachmann/openerp/commit/671bda4d555fac1622bfeab0d7b3f046c0721e07))

## [0.1.43](https://github.com/hansjlachmann/openerp/compare/v0.1.42...v0.1.43) (2026-02-12)


### Bug Fixes

* handle missing PDF path when startjob times out on heavy reports ([807bf0a](https://github.com/hansjlachmann/openerp/commit/807bf0a155499bc3831606ee9c01b814e4705c78))
* increase HTTP client and poll timeout to 60 minutes for heavy reports ([63da437](https://github.com/hansjlachmann/openerp/commit/63da437402c1c3c81de8d39a3381b082a067e363))

## [0.1.42](https://github.com/hansjlachmann/openerp/compare/v0.1.41...v0.1.42) (2026-02-12)


### Bug Fixes

* send codeunit errors as error events instead of success completion ([9ca5cda](https://github.com/hansjlachmann/openerp/commit/9ca5cda803afa895b00c333a253bc00cde195a21))

## [0.1.41](https://github.com/hansjlachmann/openerp/compare/v0.1.40...v0.1.41) (2026-02-12)


### Bug Fixes

* show NAV errors as toast instead of briefly-visible modal ([300ce9b](https://github.com/hansjlachmann/openerp/commit/300ce9bd8b5f1cc274160f1a6f202c6aa27b190e))

## [0.1.40](https://github.com/hansjlachmann/openerp/compare/v0.1.39...v0.1.40) (2026-02-12)


### Features

* add parameter field to Job Queue for generic report runner ([1edbef9](https://github.com/hansjlachmann/openerp/commit/1edbef95e96c9ecf795109351a8e1b5f2849f050))


### Bug Fixes

* don't stop polling when startjob POST times out on heavy reports ([8d55f2d](https://github.com/hansjlachmann/openerp/commit/8d55f2d8b109207043232aba3c7a4e3014d25213))
* remove ineffectual assignments flagged by golangci-lint ([2497c0b](https://github.com/hansjlachmann/openerp/commit/2497c0b2e02c2b582f68dd2f7fbb68ffef05e1fb))
* stop polling when startjob fails and no progress is ever seen ([7614555](https://github.com/hansjlachmann/openerp/commit/76145553911094149394ea3d9eda5eeee3dc87cb))

## [0.1.39](https://github.com/hansjlachmann/openerp/compare/v0.1.38...v0.1.39) (2026-02-12)


### Bug Fixes

* show 0% progress when report job starts ([b8d6f8c](https://github.com/hansjlachmann/openerp/commit/b8d6f8cdf5f431e1af50c18333163b292d1b72dc))

## [0.1.38](https://github.com/hansjlachmann/openerp/compare/v0.1.37...v0.1.38) (2026-02-12)


### Bug Fixes

* extract PDF path from StartJob response, not CheckJob ([76ead05](https://github.com/hansjlachmann/openerp/commit/76ead05af464ee594f871e8fb41f762914144458))

## [0.1.37](https://github.com/hansjlachmann/openerp/compare/v0.1.36...v0.1.37) (2026-02-12)


### Bug Fixes

* correct PDF endpoint URL and add PdfPath parameter ([fa905da](https://github.com/hansjlachmann/openerp/commit/fa905dad6d03aaf00ada5f173332484b486fed48))

## [0.1.36](https://github.com/hansjlachmann/openerp/compare/v0.1.35...v0.1.36) (2026-02-12)


### Bug Fixes

* add initial delay before checkjob poll and fix PDF download endpoint ([1d80f8c](https://github.com/hansjlachmann/openerp/commit/1d80f8ce090249f13191356fff40725ffe42ec80))
* update CheckJob call to POST with CompanyName parameter ([f4b3001](https://github.com/hansjlachmann/openerp/commit/f4b30010522bc0493908bb422c7bc7dfbe8acfb4))

## [0.1.35](https://github.com/hansjlachmann/openerp/compare/v0.1.34...v0.1.35) (2026-02-11)


### Features

* move all frontend translations to backend YAML files ([53f180a](https://github.com/hansjlachmann/openerp/commit/53f180aa003c4b259ac406a4ab76fddcf2546343))
* render boolean fields as checkboxes on card pages ([8ac1a1b](https://github.com/hansjlachmann/openerp/commit/8ac1a1b884322a1884e18ffddcc4b19ecc5925c4))


### Bug Fixes

* correct Norwegian menu item translations ([5a4d152](https://github.com/hansjlachmann/openerp/commit/5a4d1524ad6bac54c4eeb755ccdf16b9f63702a5))
* update E2E test to accept translation keys when backend is unavailable ([3d5f48e](https://github.com/hansjlachmann/openerp/commit/3d5f48eb110c216f43f5cb58238574017c26f048))
* use full page reload after login to apply user's language ([c0080f9](https://github.com/hansjlachmann/openerp/commit/c0080f9d7c3ee62d40c45c14fcd653c7ada0fafd))

## [0.1.34](https://github.com/hansjlachmann/openerp/compare/v0.1.33...v0.1.34) (2026-02-11)


### Features

* add JOBQUEUE menu profile and fix nb-NO translation gaps ([75d8f69](https://github.com/hansjlachmann/openerp/commit/75d8f698e1a4abc0c267680654a602031c550690))
* extract CreateJobQueueEntry helper, add menu i18n, and fix missing translations ([62996de](https://github.com/hansjlachmann/openerp/commit/62996de2b2030b2f0e5d969d2997060cc66d8803))

## [0.1.33](https://github.com/hansjlachmann/openerp/compare/v0.1.32...v0.1.33) (2026-02-10)


### Bug Fixes

* default empty string to first option for Option fields (behavior) ([7bc1fe2](https://github.com/hansjlachmann/openerp/commit/7bc1fe2eac4702b4fb70759fc15dafc7c06f01c9))

## [0.1.32](https://github.com/hansjlachmann/openerp/compare/v0.1.31...v0.1.32) (2026-02-10)


### Features

* send company name in NavReportRunner startjob request ([d5654de](https://github.com/hansjlachmann/openerp/commit/d5654de915813c20fa2e8a72f3e3fda12d76876c))

## [0.1.31](https://github.com/hansjlachmann/openerp/compare/v0.1.30...v0.1.31) (2026-02-10)


### Bug Fixes

* update .env version and prevent CI race condition in .env commit step ([2a88436](https://github.com/hansjlachmann/openerp/commit/2a88436008ca90ef41ea3df549e5a011bf6f6522))

## [0.1.30](https://github.com/hansjlachmann/openerp/compare/v0.1.29...v0.1.30) (2026-02-10)


### Features

* add APP_VERSION support to docker-compose ([5b220a9](https://github.com/hansjlachmann/openerp/commit/5b220a9469997f8d877042b1518c93c7b88cd9ad))
* add automatic table relation validation in tablegen ([8718bab](https://github.com/hansjlachmann/openerp/commit/8718babbe8daf4a83d95ccc6c4b0cba098eff1cf))
* add cancel button to progress modal for long-running jobs ([9b26b5f](https://github.com/hansjlachmann/openerp/commit/9b26b5f924f6a90155c536365e52ac4f47e7cf89))
* add codeunit helper functions Message() and Error() ([bccd1b4](https://github.com/hansjlachmann/openerp/commit/bccd1b463a31940798be775e3d52183a718a690d))
* add codeunit to generate random customer ledger entries ([6e9ab20](https://github.com/hansjlachmann/openerp/commit/6e9ab206e57c0dbd8ada44bb131de07dffd072d6))
* add company switcher and codeunit dialog support ([32b4453](https://github.com/hansjlachmann/openerp/commit/32b445330c0e2264c85f241225630fd0f626ce4f))
* add Confirm() helper function for codeunits ([9900f57](https://github.com/hansjlachmann/openerp/commit/9900f57257b1263b3c179716579b6a9bda37f8bb))
* add dark mode support to login page and layout ([16ff6e4](https://github.com/hansjlachmann/openerp/commit/16ff6e471395939c4ee1fcebe79092b141d2b03b))
* add detailed logging to NavReportRunner for debugging ([53d10c9](https://github.com/hansjlachmann/openerp/commit/53d10c90f94e7d816c5b1d976dd77eb0e6ccbbe2))
* add Escape key navigation in list pages (behavior) ([566d210](https://github.com/hansjlachmann/openerp/commit/566d2102bd14cea552d2e0babce69ec6f867b77a))
* add extension support with extmerge tool ([e9e173d](https://github.com/hansjlachmann/openerp/commit/e9e173ddf64e6267b0000bccda6a11e59b744982))
* add F8 to copy value from cell above (behavior) ([571e663](https://github.com/hansjlachmann/openerp/commit/571e66362d59e07f18c7ac9b541777cdff82e02b))
* add focus_field property for Card pages ([8a33e1f](https://github.com/hansjlachmann/openerp/commit/8a33e1f258be2a37e726b466e6853e2232281ec2))
* add i18n for messages and display company name in menu bar ([b55865e](https://github.com/hansjlachmann/openerp/commit/b55865e89f4328afd456b70408f1dd5262915be8))
* add i18n for messages and display company name in menu bar ([b6df85f](https://github.com/hansjlachmann/openerp/commit/b6df85fc949ce74a7b17c44f040be4e09579da13))
* add Job Queue Entry table and list page ([1a5b4a1](https://github.com/hansjlachmann/openerp/commit/1a5b4a1ee9fb991d87734b02ccc7f905c2698531))
* add Job Queue table with Run action and code optimizations ([9ea996e](https://github.com/hansjlachmann/openerp/commit/9ea996e8f6dbbcc9b578f5db5c22d421094b9f10))
* add keyboard shortcuts for List page actions ([8136ead](https://github.com/hansjlachmann/openerp/commit/8136eaddcea96f41ac9c051a47580bed13c7f0d0))
* add Language table with relation to User ([96f9447](https://github.com/hansjlachmann/openerp/commit/96f94476d6791c8a795ed9063f5ebe52b9531ce1))
* add logging for POST request to NAV service ([87270f3](https://github.com/hansjlachmann/openerp/commit/87270f382aa3fa8ef0061f84bd92d56bc9d6cabb))
* Add logout functionality and enforce authentication ([30e8deb](https://github.com/hansjlachmann/openerp/commit/30e8deb35fd11a80be9d9e33070506daec7b0d31))
* add multi-column lookup dropdown with type-ahead search ([17a7fec](https://github.com/hansjlachmann/openerp/commit/17a7fecc573e0543abd8b2d9f545ac5aa9b12343))
* add multi-language support and breadcrumb navigation ([6e8732d](https://github.com/hansjlachmann/openerp/commit/6e8732d1252b9a1eca19dd96d7ff1f794b0f0d7f))
* add progress dialog for codeunits ([43e1c1f](https://github.com/hansjlachmann/openerp/commit/43e1c1f7163b0c7d99784a845461f06f3b0c5af2))
* add NavReportRunner codeunit for external report generation ([5ff677c](https://github.com/hansjlachmann/openerp/commit/5ff677c49ce787512618d2ab93e1a0f125971689))
* add Option field support and improve modal UX ([fc34ac2](https://github.com/hansjlachmann/openerp/commit/fc34ac27bcd3aa13de97f706e708f626fee017d7))
* add permission enforcement middleware for table API routes ([6b1a26f](https://github.com/hansjlachmann/openerp/commit/6b1a26fea71b02f34a47499208be1d2dd290b69f))
* add Permission table and session-based RBAC ([de2b6ec](https://github.com/hansjlachmann/openerp/commit/de2b6ecd82df5f14f193c9d5ababd6eb8ed13516))
* add production docker-compose with pre-built images ([5a8286c](https://github.com/hansjlachmann/openerp/commit/5a8286c4dc4c5f6486d8656d69683e635327cd60))
* add session helper functions to codeunits package ([75e8815](https://github.com/hansjlachmann/openerp/commit/75e88156e9763d92e1882cc07082a7978593624b))
* add table relation validation with field revert on error ([519571b](https://github.com/hansjlachmann/openerp/commit/519571bcacdb50462e7ce011599c65737767d5d5))
* add translation_key field and fix list page empty row handling ([4f852a5](https://github.com/hansjlachmann/openerp/commit/4f852a5e98a4e7a6c022bdda3854f3f8b603030f))
* add translations for permission tables and seed default roles ([0eae0de](https://github.com/hansjlachmann/openerp/commit/0eae0ded24ec2a28687eda4a1ac3ec0bd7a9cb68))
* add UI components and improve editable list functionality ([5b1c5cc](https://github.com/hansjlachmann/openerp/commit/5b1c5ccd8c0d7b7864d75f087c500ba26625643a))
* Add user authentication and management system ([2e84c06](https://github.com/hansjlachmann/openerp/commit/2e84c06de4e4bcb6f761d64a8920cf1511f0b7a3))
* Add user preferences system and filter support ([9deb19b](https://github.com/hansjlachmann/openerp/commit/9deb19ba0464c161d0b81b7725ead4e4adda0722))
* add User Role and User Member permission tables ([d4c2e61](https://github.com/hansjlachmann/openerp/commit/d4c2e61bbeeb020ff527ca98f63ddc01405f8d2f))
* add User Role card page and Permission list page ([15a9c0c](https://github.com/hansjlachmann/openerp/commit/15a9c0cefe363026513ac63de109cfe1604eb0a6))
* Add user-specific customizations and fix phone number field ([d2d2b75](https://github.com/hansjlachmann/openerp/commit/d2d2b756f366018dad605ecacfba2a6976671cbf))
* add versioned database migration system ([42c51af](https://github.com/hansjlachmann/openerp/commit/42c51aff5735f0dacc39ef1d5772ccceec53ca8b))
* auto-update .env with version on release ([d505c9a](https://github.com/hansjlachmann/openerp/commit/d505c9aaf9ebbb4165fa63b6533b5197fd36e5ae))
* block editing when new record save fails (e.g., duplicate) ([9b18432](https://github.com/hansjlachmann/openerp/commit/9b18432d300176f5fbea5e9d0687f7f6d753c363))
* codeunits self-declare progress support via UsesProgress() ([d696acc](https://github.com/hansjlachmann/openerp/commit/d696acc8252daa13240ac8f382eca6ff8fa4879c))
* display version in menu bar ([3974c28](https://github.com/hansjlachmann/openerp/commit/3974c28f9a54bfe388913962a5dac321d929dff8))
* Docker containerization with PostgreSQL and UI improvements ([de9f562](https://github.com/hansjlachmann/openerp/commit/de9f56258cba6ae88bfca91b32ee20382cbef3b6))
* Docker containerization with PostgreSQL and UI improvements ([cde09e8](https://github.com/hansjlachmann/openerp/commit/cde09e89ddb92c9a01f62e2d2a4a8c11d0d71cae))
* enforce company access control, composite PK support, and global table sync ([5ddf649](https://github.com/hansjlachmann/openerp/commit/5ddf64983bf0a13afa5821659a72fedac0f10344))
* fire-and-forget POST to NAV service, poll immediately ([aafcae7](https://github.com/hansjlachmann/openerp/commit/aafcae71aa321849e672f357725644120642e8c9))
* Front-end UI ([c2f1f59](https://github.com/hansjlachmann/openerp/commit/c2f1f594c8e46bd2ae048b2c156bb34bb9fb2849))
* generate 20-char alphanumeric JobId with timestamp ([f237266](https://github.com/hansjlachmann/openerp/commit/f2372663eaef658e7560c25e1dd05bcb0ef802f1))
* genereric error messages from foundation layer ([41cc54f](https://github.com/hansjlachmann/openerp/commit/41cc54f64ca2e0e8960551dde42fd87810cef624))
* implement generic codeunit registry pattern ([a499448](https://github.com/hansjlachmann/openerp/commit/a499448f660408579921d6d0a0edbdcf50d9bcff))
* implement user-assignable menu system ([b1acd0a](https://github.com/hansjlachmann/openerp/commit/b1acd0a538b610d4176dba258ebd24a235138147))
* improve Customer Card modal UX and Edit button functionality ([1d3c6b3](https://github.com/hansjlachmann/openerp/commit/1d3c6b3ecf90ef3bc34f7c0e7929556126f1242a))
* improve Customer Card UX and fix button states ([513fdaa](https://github.com/hansjlachmann/openerp/commit/513fdaa057b38f3a0f45591c3fb09aeab60cfb1d))
* improve list page edit mode keyboard navigation (behavior) ([86a9dee](https://github.com/hansjlachmann/openerp/commit/86a9dee37d3afdc433de08f97dc827b6202ac6e5))
* improve report polling - check PDF endpoint every 5 seconds ([0919bbf](https://github.com/hansjlachmann/openerp/commit/0919bbfe3ef9aa6573408d9751888295c0c4acd5))
* make menu groups data-driven from YAML ([97f76f5](https://github.com/hansjlachmann/openerp/commit/97f76f58929cb9b92c85a3aa29865c023d5a7fea))
* Redesign FilterPane with classic ERP-style Views ([1de6219](https://github.com/hansjlachmann/openerp/commit/1de621939fd504cc9db068edc9fc62748011ff89))
* Reorganize page header buttons layout ([e6bcb03](https://github.com/hansjlachmann/openerp/commit/e6bcb03594931e3bd95d8b4d3aed6ffa3a0612d7))
* show lookup dropdowns in non-edit mode on card pages ([019d4a7](https://github.com/hansjlachmann/openerp/commit/019d4a70c8753c47e5dc8dbccf2f8ddf97b1074d))
* smart auto-save with change detection and UI improvements ([4687664](https://github.com/hansjlachmann/openerp/commit/468766481878baf1bc6ddabf6ff5b364d2fbed6d))
* UI components, multi-language support, and editable list improvements ([#17](https://github.com/hansjlachmann/openerp/issues/17)) ([0391eb4](https://github.com/hansjlachmann/openerp/commit/0391eb4ffaf046b0e24889167be19804b9276943))
* update NavReportRunner to use dedicated PDF endpoint ([b4d6305](https://github.com/hansjlachmann/openerp/commit/b4d6305f5c3cc5d7a505390e6b6ac29450af3067))
* use LookupDropdown in list page edit cells, fix composite PK SQL ([21785b4](https://github.com/hansjlachmann/openerp/commit/21785b4c41c28fb43a21ce72e7805c1cd4ae613a))


### Bug Fixes

* add Back to List action to User Card ([9e782f1](https://github.com/hansjlachmann/openerp/commit/9e782f1f9e89cc68d31865b95d318a550c492839))
* add empty ID validation to ModifyRecord and DeleteRecord ([e2d9492](https://github.com/hansjlachmann/openerp/commit/e2d9492f435417605e83299154549874287e7a68))
* add empty ID validation to ModifyRecord and DeleteRecord ([9b1a341](https://github.com/hansjlachmann/openerp/commit/9b1a341b8ac7652559a98629e7facf8476bbeb06))
* Add ensureTableExists to create tables on-demand from metadata ([8241e27](https://github.com/hansjlachmann/openerp/commit/8241e277b1caf96809f67adb345029bd2642d651))
* Add missing caption for Payment Terms 'active' field ([7c48f2b](https://github.com/hansjlachmann/openerp/commit/7c48f2ba8acfdd1c5b6945b2c7b337594e3c9856))
* Add missing fyne.io/fyne/v2 import for GUI ([be2ffe6](https://github.com/hansjlachmann/openerp/commit/be2ffe63eea44fb781cb13a60ecb1f7693193ba8))
* add table_relation to language field in User card page ([f8704c9](https://github.com/hansjlachmann/openerp/commit/f8704c9bade4dcd7f32572048b742b04560db604))
* allow CORS from all origins in production ([37f9d62](https://github.com/hansjlachmann/openerp/commit/37f9d62d2a3964c50e3ecca304122ebd9cc111d1))
* card page keyboard shortcuts and dark mode default ([c651582](https://github.com/hansjlachmann/openerp/commit/c65158207c625b390cdc62d8098e6cb68435fefb))
* consistent row height between edit and view mode in list pages ([7a6482a](https://github.com/hansjlachmann/openerp/commit/7a6482a29ee2ce18d9e7d7487efe36524804a8a5))
* correct Norwegian translation for preferences ([824ae69](https://github.com/hansjlachmann/openerp/commit/824ae691e1706167c4f0c61834d4edacf4d0b26d))
* correct TypeScript type for lookup data in getTableOptionsAndLookups ([582be24](https://github.com/hansjlachmann/openerp/commit/582be245febca6ccbe5a09203b43f72fd42b6bbd))
* Create FieldDefinition table in OpenDatabase for backward compatibility ([abd18c6](https://github.com/hansjlachmann/openerp/commit/abd18c6daa3f4e1ce7355b9604c4797bb5332bf0))
* CreateTable now inserts marker record so table shows in ListTables ([2cffb52](https://github.com/hansjlachmann/openerp/commit/2cffb52bd57923cceb61c83156c443166d2e6dd7))
* day and night mode ([6fe82e6](https://github.com/hansjlachmann/openerp/commit/6fe82e604a65259e06029dd58c021a5dbdd72511))
* delayed insert for composite PKs with optional fields ([58f9282](https://github.com/hansjlachmann/openerp/commit/58f9282c88bc5593477e7195d37b8c9fcbac88bd))
* focus first input when opening card page modal ([000475a](https://github.com/hansjlachmann/openerp/commit/000475a97d8daab8106c91b653e3d87b9c20fdf6))
* handle NULL database values and improve new record detection ([4b0da75](https://github.com/hansjlachmann/openerp/commit/4b0da7528e201b9df7542257338c85ade289d218))
* hardcode report ID to 121 for NAV proxy service ([fd5b79c](https://github.com/hansjlachmann/openerp/commit/fd5b79c84fdb09a8b9f29007860292db656d7ca4))
* improve progress polling - poll immediately and robust parsing ([11515b6](https://github.com/hansjlachmann/openerp/commit/11515b63b73a63b567b445a0ef3a3ccd0459707e))
* improve version text visibility in menu bar ([c7161a5](https://github.com/hansjlachmann/openerp/commit/c7161a5d77fb90f18ed5ff0af520672a9a2f7500))
* list page border now ends at last record ([7f523a5](https://github.com/hansjlachmann/openerp/commit/7f523a5c26c347740875f7922077217643161dd8))
* make list page column headers sticky when scrolling ([9b538b7](https://github.com/hansjlachmann/openerp/commit/9b538b75e3b163f0f289220ece2bfbc65ad11955))
* make list page column headers sticky when scrolling ([c351a47](https://github.com/hansjlachmann/openerp/commit/c351a47a486af7a2c4bb8cda948acdce4ef0e8e9))
* make list page column headers sticky when scrolling ([0181231](https://github.com/hansjlachmann/openerp/commit/01812316f227a3c172bdfa4fc5cbf84e7bfa46a9))
* modal card close functionality and keyboard shortcut handling ([800db71](https://github.com/hansjlachmann/openerp/commit/800db71e0aca0c0d88755fb063200cdb55475947))
* modal card close functionality and keyboard shortcut handling ([eb0f5e7](https://github.com/hansjlachmann/openerp/commit/eb0f5e7edbb66fbdad153615e9f842a63d1550f1))
* Move authentication check to layout load function ([76f185d](https://github.com/hansjlachmann/openerp/commit/76f185deb584045ab8b14eabc976fba6fec60922))
* pass database type to codeunit for PostgreSQL compatibility ([baf0110](https://github.com/hansjlachmann/openerp/commit/baf01101dd812d7ad679588717f961a20eaf9ebe))
* prevent creating records with empty primary key ([fa3b505](https://github.com/hansjlachmann/openerp/commit/fa3b505fd318e0a96b706e4e12cacf127186da42))
* prevent creating records with empty primary key ([2291e06](https://github.com/hansjlachmann/openerp/commit/2291e061a52a45016b3a589c1da11fb1e1d29ba9))
* prevent duplicate new rows and allow typing in lookup fields ([d0ddde8](https://github.com/hansjlachmann/openerp/commit/d0ddde85eb6b63a1a395721be707988d3a0e41d6))
* readme ([5ec58e4](https://github.com/hansjlachmann/openerp/commit/5ec58e45e03bf6b362321bb905b6ad4307635988))
* readme ([a48c83c](https://github.com/hansjlachmann/openerp/commit/a48c83c0745cb69ce62f9eb95dc8f7818530bf09))
* reduce progress polling interval to 1 second ([99f8095](https://github.com/hansjlachmann/openerp/commit/99f8095b249b52351543edb66cf944e12ca8006b))
* reduce row height for option dropdowns in list page ([ac2335f](https://github.com/hansjlachmann/openerp/commit/ac2335f9e26cb606f23b04b7ecc9fa2e663035fe))
* register Language table in table registry ([b389fa6](https://github.com/hansjlachmann/openerp/commit/b389fa6154831ef13ec688a35e7285c547ac3c4e))
* Remove nested go.mod files and update import paths ([eff6084](https://github.com/hansjlachmann/openerp/commit/eff60847667ecb0edd4406c51e47a22b32b5694a))
* remove unused confirmChan field from Dialog struct ([ad175aa](https://github.com/hansjlachmann/openerp/commit/ad175aae942eca89ffbe6691e45404b37b94cfd8))
* remove unused escapeJSON function ([47331e7](https://github.com/hansjlachmann/openerp/commit/47331e72c11f2b239cca6d3853999f3eacb0e568))
* resolve go vet errors in backend code ([3baf769](https://github.com/hansjlachmann/openerp/commit/3baf769d3ffaae7c7a9a8f6f4d7da9d286cf0e1d))
* resolve golangci-lint errors ([0d3ce8e](https://github.com/hansjlachmann/openerp/commit/0d3ce8e535d49a7302ea19f885d2b9472200b0aa))
* resolve remaining a11y warnings ([f50e8f8](https://github.com/hansjlachmann/openerp/commit/f50e8f895e41f7c4b48a5a0baad6c70637cbafba))
* resolve Svelte 5 props_invalid_value error for lookup dropdowns ([9c1ca22](https://github.com/hansjlachmann/openerp/commit/9c1ca221c735e0cbf1673b4aee660a9bb2e023ab))
* resolve Svelte 5 warnings ([7189359](https://github.com/hansjlachmann/openerp/commit/71893592eaaaa4643d8d7e078076b2817b7da1ed))
* resolve svelte-check warnings ([b5052f0](https://github.com/hansjlachmann/openerp/commit/b5052f01340adf58a0fb48304549b0285f36c2df))
* resolve TypeScript errors in frontend build ([3e5345f](https://github.com/hansjlachmann/openerp/commit/3e5345f52761d139ac2dfe96c44c2c3777220b16))
* resolve unreachable code errors in generated table files ([627ec69](https://github.com/hansjlachmann/openerp/commit/627ec694a0775e9f832ae89d609f8fb3003adeaa))
* support composite primary keys in get/modify/delete API endpoints ([3cd6da1](https://github.com/hansjlachmann/openerp/commit/3cd6da1308970b00aa6d6064c8e8f245a61c85dc))
* support HTTP (non-secure) contexts for toast notifications ([db28331](https://github.com/hansjlachmann/openerp/commit/db2833162c3ff67aec34c3889325c91b2bf6df5f))
* update gitignore ([f6df9f6](https://github.com/hansjlachmann/openerp/commit/f6df9f6f1344ae6057a9d1e49c9b42ec6190d3ee))
* update report dialog ([c045c1a](https://github.com/hansjlachmann/openerp/commit/c045c1aafa47c8a6378ce2e7e9c37257f8b1c88c))
* use crypto/rand for job IDs and propagate POST errors ([5a697f6](https://github.com/hansjlachmann/openerp/commit/5a697f691f629b0b30ace65774ff89863765e864))
* use generic action button captions ([55ad1ce](https://github.com/hansjlachmann/openerp/commit/55ad1cef1c2a70a24f24c4061a64089d800cd91d))
* use getRecordId helper for delete and row click in PageRenderer ([4cd7a81](https://github.com/hansjlachmann/openerp/commit/4cd7a81fe9acbd42fb7908331df476cff38ae4a9))
* use per-goroutine session context for codeunit execution ([bc92085](https://github.com/hansjlachmann/openerp/commit/bc920859961fce140581ad7b21f71cdc2f638fb6))
* wait for CheckJob 100% before fetching PDF ([a7da049](https://github.com/hansjlachmann/openerp/commit/a7da0493a1a9716f8636f42bb4cf0b41d30b7a56))


### Code Refactoring

* Auto-initialize tables and remove Object Designer ([2d21d31](https://github.com/hansjlachmann/openerp/commit/2d21d31af1b2906b5acd691e93d420ea7ee6b45f))
* consolidate confirmation modal to shared store ([a7d76bd](https://github.com/hansjlachmann/openerp/commit/a7d76bd9eb352a09e456dea3c067971c5aff9326))
* consolidate duplicate code across frontend ([9a83a16](https://github.com/hansjlachmann/openerp/commit/9a83a168890c0c384ccb07757500e75d101936a1))
* consolidate duplicate code and improve code organization ([8e5b130](https://github.com/hansjlachmann/openerp/commit/8e5b13087aa5797b8c0f826a288351bb1b59e30d))
* create centralized localStorage utility ([ae3bfa6](https://github.com/hansjlachmann/openerp/commit/ae3bfa6e8502cccd8ee7462968d1fa1f9efb3b79))
* Extract duplicate code into reusable utilities and components ([fdcff3b](https://github.com/hansjlachmann/openerp/commit/fdcff3bc83c181343592e8edba334bbc8a9e8e6a))
* extract shared utilities for record handling and API helpers ([62fb453](https://github.com/hansjlachmann/openerp/commit/62fb4530e52ab6d52dc894cccb10326ef058843d))
* extract visibility logic and remove dead code ([d7812be](https://github.com/hansjlachmann/openerp/commit/d7812bec1b2f7867dae7f4023c98b62bdaea10a3))
* implement generic Table interface for API handlers ([fe4bf7f](https://github.com/hansjlachmann/openerp/commit/fe4bf7f0491a7033e47cbcc9f0aa55c52aead951))
* make frontend fully generic using primary_key from page definitions ([c50fc81](https://github.com/hansjlachmann/openerp/commit/c50fc81499e0ad98c0a86d41a7ce70cb37e8003b))
* Move pages folder to business logic layer ([b839a2e](https://github.com/hansjlachmann/openerp/commit/b839a2e5e2e26eccf980d608ef9b3abf1f28be31))
* remove legacy hardcoded customer code ([b7048cd](https://github.com/hansjlachmann/openerp/commit/b7048cdb53b8290a1467478dcd11ae284a70ae56))
* separate generated table code from manual business logic ([35b5b2b](https://github.com/hansjlachmann/openerp/commit/35b5b2b6a5fc83b708ab4066ffa7f8a0ec939461))
* translate all hardcoded API error messages ([f61fbf8](https://github.com/hansjlachmann/openerp/commit/f61fbf8c6c025617d42f7a9b067e9eb0d7294815))
* translate all hardcoded API error messages ([eff5574](https://github.com/hansjlachmann/openerp/commit/eff557482ff0a3d9e079e60e811548be584754ee))


### Documentation

* add CLAUDE.md with project rules and conventions ([44ff6df](https://github.com/hansjlachmann/openerp/commit/44ff6dfd2ee79beaaa6b7653c1c57373bfdb9eef))
* add migration system documentation ([9ddcad6](https://github.com/hansjlachmann/openerp/commit/9ddcad6184a97f92b408df237c7529a5ab13e420))
* add required runtimes Go 1.24 and Node.js 22 to CLAUDE.md ([48e2298](https://github.com/hansjlachmann/openerp/commit/48e2298cc97e32c65af0a743359341c5c9c94708))
* add Svelte 5 runes and i18n rules to CLAUDE.md ([f0f1175](https://github.com/hansjlachmann/openerp/commit/f0f117545c1abd093b3cee450928aafa1b3421a8))


### CI/CD

* add code coverage with Codecov and frontend tests ([0007791](https://github.com/hansjlachmann/openerp/commit/0007791a3a2643209cdbd000498d2d73517aeb2f))
* add Docker build and push to GitHub Container Registry ([75d4999](https://github.com/hansjlachmann/openerp/commit/75d4999c82f8ec65ca675eb89ddc1afc86705b65))
* add GitHub Actions build and lint workflow ([6585658](https://github.com/hansjlachmann/openerp/commit/658565825396651ba7ac25d698a3746cc2ac7113))
* add Go test step with race detection and coverage ([5b6e03c](https://github.com/hansjlachmann/openerp/commit/5b6e03c37bd06c69e3b768b66a59efd60c39d616))
* add golangci-lint step ([8fd3ca7](https://github.com/hansjlachmann/openerp/commit/8fd3ca780766acce15420d949680080bc62fbaba))
* add multi-arch Docker builds (AMD64 + ARM64) ([0a26de2](https://github.com/hansjlachmann/openerp/commit/0a26de233d57b0058a8c8804d9c3b688f40e2846))
* add Playwright E2E testing ([f444392](https://github.com/hansjlachmann/openerp/commit/f444392c401507cfee5f1cbe58df0d152e8bd55a))
* add release-please for automated releases ([ffd6cdd](https://github.com/hansjlachmann/openerp/commit/ffd6cdd07f8c85b7633b7f9f19d7400831ce140d))
* added "needs" to release-please ([a2c48a5](https://github.com/hansjlachmann/openerp/commit/a2c48a5eadd754e738a9b8fed28e629dfcdba0ec))
* merge build and release workflows into single file ([49d8819](https://github.com/hansjlachmann/openerp/commit/49d8819266397e4afd1352e2dee5bafb5821e0c4))
* only build Docker images on release ([a911fcb](https://github.com/hansjlachmann/openerp/commit/a911fcb4bdc380cde7f028955b549ecf8c527822))

## [0.1.29](https://github.com/hansjlachmann/openerp/compare/v0.1.28...v0.1.29) (2026-02-07)


### Features

* add permission enforcement middleware for table API routes ([6b1a26f](https://github.com/hansjlachmann/openerp/commit/6b1a26fea71b02f34a47499208be1d2dd290b69f))
* add Permission table and session-based RBAC ([de2b6ec](https://github.com/hansjlachmann/openerp/commit/de2b6ecd82df5f14f193c9d5ababd6eb8ed13516))
* add translations for permission tables and seed default roles ([0eae0de](https://github.com/hansjlachmann/openerp/commit/0eae0ded24ec2a28687eda4a1ac3ec0bd7a9cb68))
* add User Role and User Member permission tables ([d4c2e61](https://github.com/hansjlachmann/openerp/commit/d4c2e61bbeeb020ff527ca98f63ddc01405f8d2f))
* add User Role card page and Permission list page ([15a9c0c](https://github.com/hansjlachmann/openerp/commit/15a9c0cefe363026513ac63de109cfe1604eb0a6))


### Documentation

* add CLAUDE.md with project rules and conventions ([44ff6df](https://github.com/hansjlachmann/openerp/commit/44ff6dfd2ee79beaaa6b7653c1c57373bfdb9eef))
* add required runtimes Go 1.24 and Node.js 22 to CLAUDE.md ([48e2298](https://github.com/hansjlachmann/openerp/commit/48e2298cc97e32c65af0a743359341c5c9c94708))
* add Svelte 5 runes and i18n rules to CLAUDE.md ([f0f1175](https://github.com/hansjlachmann/openerp/commit/f0f117545c1abd093b3cee450928aafa1b3421a8))

## [0.1.28](https://github.com/hansjlachmann/openerp/compare/v0.1.27...v0.1.28) (2026-02-06)


### Bug Fixes

* readme ([5ec58e4](https://github.com/hansjlachmann/openerp/commit/5ec58e45e03bf6b362321bb905b6ad4307635988))
* readme ([a48c83c](https://github.com/hansjlachmann/openerp/commit/a48c83c0745cb69ce62f9eb95dc8f7818530bf09))

## [0.1.27](https://github.com/hansjlachmann/openerp/compare/v0.1.26...v0.1.27) (2026-02-02)


### Bug Fixes

* use crypto/rand for job IDs and propagate POST errors ([5a697f6](https://github.com/hansjlachmann/openerp/commit/5a697f691f629b0b30ace65774ff89863765e864))
* wait for CheckJob 100% before fetching PDF ([a7da049](https://github.com/hansjlachmann/openerp/commit/a7da0493a1a9716f8636f42bb4cf0b41d30b7a56))

## [0.1.26](https://github.com/hansjlachmann/openerp/compare/v0.1.25...v0.1.26) (2026-01-29)


### Features

* add logging for POST request to NAV service ([87270f3](https://github.com/hansjlachmann/openerp/commit/87270f382aa3fa8ef0061f84bd92d56bc9d6cabb))
* fire-and-forget POST to NAV service, poll immediately ([aafcae7](https://github.com/hansjlachmann/openerp/commit/aafcae71aa321849e672f357725644120642e8c9))
* improve report polling - check PDF endpoint every 5 seconds ([0919bbf](https://github.com/hansjlachmann/openerp/commit/0919bbfe3ef9aa6573408d9751888295c0c4acd5))


### Bug Fixes

* hardcode report ID to 121 for NAV proxy service ([fd5b79c](https://github.com/hansjlachmann/openerp/commit/fd5b79c84fdb09a8b9f29007860292db656d7ca4))
* reduce progress polling interval to 1 second ([99f8095](https://github.com/hansjlachmann/openerp/commit/99f8095b249b52351543edb66cf944e12ca8006b))

## [0.1.25](https://github.com/hansjlachmann/openerp/compare/v0.1.24...v0.1.25) (2026-01-29)


### Features

* add detailed logging to NavReportRunner for debugging ([53d10c9](https://github.com/hansjlachmann/openerp/commit/53d10c90f94e7d816c5b1d976dd77eb0e6ccbbe2))

## [0.1.24](https://github.com/hansjlachmann/openerp/compare/v0.1.23...v0.1.24) (2026-01-28)


### Features

* add cancel button to progress modal for long-running jobs ([9b26b5f](https://github.com/hansjlachmann/openerp/commit/9b26b5f924f6a90155c536365e52ac4f47e7cf89))


### Bug Fixes

* improve progress polling - poll immediately and robust parsing ([11515b6](https://github.com/hansjlachmann/openerp/commit/11515b63b73a63b567b445a0ef3a3ccd0459707e))

## [0.1.23](https://github.com/hansjlachmann/openerp/compare/v0.1.22...v0.1.23) (2026-01-28)


### Features

* add NavReportRunner codeunit for external report generation ([5ff677c](https://github.com/hansjlachmann/openerp/commit/5ff677c49ce787512618d2ab93e1a0f125971689))
* generate 20-char alphanumeric JobId with timestamp ([f237266](https://github.com/hansjlachmann/openerp/commit/f2372663eaef658e7560c25e1dd05bcb0ef802f1))
* update NavReportRunner to use dedicated PDF endpoint ([b4d6305](https://github.com/hansjlachmann/openerp/commit/b4d6305f5c3cc5d7a505390e6b6ac29450af3067))


### Bug Fixes

* remove unused escapeJSON function ([47331e7](https://github.com/hansjlachmann/openerp/commit/47331e72c11f2b239cca6d3853999f3eacb0e568))

## [0.1.22](https://github.com/hansjlachmann/openerp/compare/v0.1.21...v0.1.22) (2026-01-28)


### Features

* add Escape key navigation in list pages (behavior) ([566d210](https://github.com/hansjlachmann/openerp/commit/566d2102bd14cea552d2e0babce69ec6f867b77a))
* add F8 to copy value from cell above (behavior) ([571e663](https://github.com/hansjlachmann/openerp/commit/571e66362d59e07f18c7ac9b541777cdff82e02b))
* improve list page edit mode keyboard navigation (behavior) ([86a9dee](https://github.com/hansjlachmann/openerp/commit/86a9dee37d3afdc433de08f97dc827b6202ac6e5))


### Bug Fixes

* consistent row height between edit and view mode in list pages ([7a6482a](https://github.com/hansjlachmann/openerp/commit/7a6482a29ee2ce18d9e7d7487efe36524804a8a5))

## [0.1.21](https://github.com/hansjlachmann/openerp/compare/v0.1.20...v0.1.21) (2026-01-28)


### Features

* add Confirm() helper function for codeunits ([9900f57](https://github.com/hansjlachmann/openerp/commit/9900f57257b1263b3c179716579b6a9bda37f8bb))


### Bug Fixes

* remove unused confirmChan field from Dialog struct ([ad175aa](https://github.com/hansjlachmann/openerp/commit/ad175aae942eca89ffbe6691e45404b37b94cfd8))

## [0.1.20](https://github.com/hansjlachmann/openerp/compare/v0.1.19...v0.1.20) (2026-01-28)


### Bug Fixes

* support HTTP (non-secure) contexts for toast notifications ([db28331](https://github.com/hansjlachmann/openerp/commit/db2833162c3ff67aec34c3889325c91b2bf6df5f))

## [0.1.19](https://github.com/hansjlachmann/openerp/compare/v0.1.18...v0.1.19) (2026-01-28)


### Features

* add production docker-compose with pre-built images ([5a8286c](https://github.com/hansjlachmann/openerp/commit/5a8286c4dc4c5f6486d8656d69683e635327cd60))

## [0.1.18](https://github.com/hansjlachmann/openerp/compare/v0.1.17...v0.1.18) (2026-01-28)


### Features

* add APP_VERSION support to docker-compose ([5b220a9](https://github.com/hansjlachmann/openerp/commit/5b220a9469997f8d877042b1518c93c7b88cd9ad))
* add progress dialog for codeunits ([43e1c1f](https://github.com/hansjlachmann/openerp/commit/43e1c1f7163b0c7d99784a845461f06f3b0c5af2))
* auto-update .env with version on release ([d505c9a](https://github.com/hansjlachmann/openerp/commit/d505c9aaf9ebbb4165fa63b6533b5197fd36e5ae))
* codeunits self-declare progress support via UsesProgress() ([d696acc](https://github.com/hansjlachmann/openerp/commit/d696acc8252daa13240ac8f382eca6ff8fa4879c))


### Bug Fixes

* allow CORS from all origins in production ([37f9d62](https://github.com/hansjlachmann/openerp/commit/37f9d62d2a3964c50e3ecca304122ebd9cc111d1))
* use per-goroutine session context for codeunit execution ([bc92085](https://github.com/hansjlachmann/openerp/commit/bc920859961fce140581ad7b21f71cdc2f638fb6))

## [0.1.17](https://github.com/hansjlachmann/openerp/compare/v0.1.16...v0.1.17) (2026-01-27)


### Documentation

* add migration system documentation ([9ddcad6](https://github.com/hansjlachmann/openerp/commit/9ddcad6184a97f92b408df237c7529a5ab13e420))

## [0.1.16](https://github.com/hansjlachmann/openerp/compare/v0.1.15...v0.1.16) (2026-01-27)


### Features

* add session helper functions to codeunits package ([75e8815](https://github.com/hansjlachmann/openerp/commit/75e88156e9763d92e1882cc07082a7978593624b))

## [0.1.15](https://github.com/hansjlachmann/openerp/compare/v0.1.14...v0.1.15) (2026-01-27)


### Features

* add codeunit helper functions Message() and Error() ([bccd1b4](https://github.com/hansjlachmann/openerp/commit/bccd1b463a31940798be775e3d52183a718a690d))

## [0.1.14](https://github.com/hansjlachmann/openerp/compare/v0.1.13...v0.1.14) (2026-01-27)


### Features

* add company switcher and codeunit dialog support ([32b4453](https://github.com/hansjlachmann/openerp/commit/32b445330c0e2264c85f241225630fd0f626ce4f))

## [0.1.13](https://github.com/hansjlachmann/openerp/compare/v0.1.12...v0.1.13) (2026-01-27)


### Features

* add Job Queue Entry table and list page ([1a5b4a1](https://github.com/hansjlachmann/openerp/commit/1a5b4a1ee9fb991d87734b02ccc7f905c2698531))

## [0.1.12](https://github.com/hansjlachmann/openerp/compare/v0.1.11...v0.1.12) (2026-01-27)


### Features

* add Job Queue table with Run action and code optimizations ([9ea996e](https://github.com/hansjlachmann/openerp/commit/9ea996e8f6dbbcc9b578f5db5c22d421094b9f10))


### Bug Fixes

* resolve unreachable code errors in generated table files ([627ec69](https://github.com/hansjlachmann/openerp/commit/627ec694a0775e9f832ae89d609f8fb3003adeaa))

## [0.1.11](https://github.com/hansjlachmann/openerp/compare/v0.1.10...v0.1.11) (2026-01-27)


### Features

* add extension support with extmerge tool ([e9e173d](https://github.com/hansjlachmann/openerp/commit/e9e173ddf64e6267b0000bccda6a11e59b744982))

## [0.1.10](https://github.com/hansjlachmann/openerp/compare/v0.1.9...v0.1.10) (2026-01-27)


### CI/CD

* only build Docker images on release ([a911fcb](https://github.com/hansjlachmann/openerp/commit/a911fcb4bdc380cde7f028955b549ecf8c527822))

## [0.1.9](https://github.com/hansjlachmann/openerp/compare/v0.1.8...v0.1.9) (2026-01-27)


### Bug Fixes

* add Back to List action to User Card ([9e782f1](https://github.com/hansjlachmann/openerp/commit/9e782f1f9e89cc68d31865b95d318a550c492839))
* card page keyboard shortcuts and dark mode default ([c651582](https://github.com/hansjlachmann/openerp/commit/c65158207c625b390cdc62d8098e6cb68435fefb))

## [0.1.8](https://github.com/hansjlachmann/openerp/compare/v0.1.7...v0.1.8) (2026-01-27)


### Features

* display version in menu bar ([3974c28](https://github.com/hansjlachmann/openerp/commit/3974c28f9a54bfe388913962a5dac321d929dff8))


### Bug Fixes

* improve version text visibility in menu bar ([c7161a5](https://github.com/hansjlachmann/openerp/commit/c7161a5d77fb90f18ed5ff0af520672a9a2f7500))

## [0.1.7](https://github.com/hansjlachmann/openerp/compare/v0.1.6...v0.1.7) (2026-01-27)


### Features

* add versioned database migration system ([42c51af](https://github.com/hansjlachmann/openerp/commit/42c51aff5735f0dacc39ef1d5772ccceec53ca8b))

## [0.1.6](https://github.com/hansjlachmann/openerp/compare/v0.1.5...v0.1.6) (2026-01-25)


### Features

* add codeunit to generate random customer ledger entries ([6e9ab20](https://github.com/hansjlachmann/openerp/commit/6e9ab206e57c0dbd8ada44bb131de07dffd072d6))
* implement generic codeunit registry pattern ([a499448](https://github.com/hansjlachmann/openerp/commit/a499448f660408579921d6d0a0edbdcf50d9bcff))


### Bug Fixes

* pass database type to codeunit for PostgreSQL compatibility ([baf0110](https://github.com/hansjlachmann/openerp/commit/baf01101dd812d7ad679588717f961a20eaf9ebe))

## [0.1.5](https://github.com/hansjlachmann/openerp/compare/v0.1.4...v0.1.5) (2026-01-25)


### Features

* make menu groups data-driven from YAML ([97f76f5](https://github.com/hansjlachmann/openerp/commit/97f76f58929cb9b92c85a3aa29865c023d5a7fea))

## [0.1.4](https://github.com/hansjlachmann/openerp/compare/v0.1.3...v0.1.4) (2026-01-24)


### CI/CD

* add multi-arch Docker builds (AMD64 + ARM64) ([0a26de2](https://github.com/hansjlachmann/openerp/commit/0a26de233d57b0058a8c8804d9c3b688f40e2846))

## [0.1.3](https://github.com/hansjlachmann/openerp/compare/v0.1.2...v0.1.3) (2026-01-23)


### CI/CD

* add Playwright E2E testing ([f444392](https://github.com/hansjlachmann/openerp/commit/f444392c401507cfee5f1cbe58df0d152e8bd55a))

## [0.1.2](https://github.com/hansjlachmann/openerp/compare/v0.1.1...v0.1.2) (2026-01-23)


### CI/CD

* add code coverage with Codecov and frontend tests ([0007791](https://github.com/hansjlachmann/openerp/commit/0007791a3a2643209cdbd000498d2d73517aeb2f))
* added "needs" to release-please ([a2c48a5](https://github.com/hansjlachmann/openerp/commit/a2c48a5eadd754e738a9b8fed28e629dfcdba0ec))

## [0.1.1](https://github.com/hansjlachmann/openerp/compare/v0.1.0...v0.1.1) (2026-01-23)


### Features

* add automatic table relation validation in tablegen ([8718bab](https://github.com/hansjlachmann/openerp/commit/8718babbe8daf4a83d95ccc6c4b0cba098eff1cf))
* add dark mode support to login page and layout ([16ff6e4](https://github.com/hansjlachmann/openerp/commit/16ff6e471395939c4ee1fcebe79092b141d2b03b))
* add focus_field property for Card pages ([8a33e1f](https://github.com/hansjlachmann/openerp/commit/8a33e1f258be2a37e726b466e6853e2232281ec2))
* add i18n for messages and display company name in menu bar ([b55865e](https://github.com/hansjlachmann/openerp/commit/b55865e89f4328afd456b70408f1dd5262915be8))
* add i18n for messages and display company name in menu bar ([b6df85f](https://github.com/hansjlachmann/openerp/commit/b6df85fc949ce74a7b17c44f040be4e09579da13))
* add keyboard shortcuts for List page actions ([8136ead](https://github.com/hansjlachmann/openerp/commit/8136eaddcea96f41ac9c051a47580bed13c7f0d0))
* add Language table with relation to User ([96f9447](https://github.com/hansjlachmann/openerp/commit/96f94476d6791c8a795ed9063f5ebe52b9531ce1))
* Add logout functionality and enforce authentication ([30e8deb](https://github.com/hansjlachmann/openerp/commit/30e8deb35fd11a80be9d9e33070506daec7b0d31))
* add multi-column lookup dropdown with type-ahead search ([17a7fec](https://github.com/hansjlachmann/openerp/commit/17a7fecc573e0543abd8b2d9f545ac5aa9b12343))
* add multi-language support and breadcrumb navigation ([6e8732d](https://github.com/hansjlachmann/openerp/commit/6e8732d1252b9a1eca19dd96d7ff1f794b0f0d7f))
* add Option field support and improve modal UX ([fc34ac2](https://github.com/hansjlachmann/openerp/commit/fc34ac27bcd3aa13de97f706e708f626fee017d7))
* add table relation validation with field revert on error ([519571b](https://github.com/hansjlachmann/openerp/commit/519571bcacdb50462e7ce011599c65737767d5d5))
* add translation_key field and fix list page empty row handling ([4f852a5](https://github.com/hansjlachmann/openerp/commit/4f852a5e98a4e7a6c022bdda3854f3f8b603030f))
* add UI components and improve editable list functionality ([5b1c5cc](https://github.com/hansjlachmann/openerp/commit/5b1c5ccd8c0d7b7864d75f087c500ba26625643a))
* Add user authentication and management system ([2e84c06](https://github.com/hansjlachmann/openerp/commit/2e84c06de4e4bcb6f761d64a8920cf1511f0b7a3))
* Add user preferences system and filter support ([9deb19b](https://github.com/hansjlachmann/openerp/commit/9deb19ba0464c161d0b81b7725ead4e4adda0722))
* Add user-specific customizations and fix phone number field ([d2d2b75](https://github.com/hansjlachmann/openerp/commit/d2d2b756f366018dad605ecacfba2a6976671cbf))
* block editing when new record save fails (e.g., duplicate) ([9b18432](https://github.com/hansjlachmann/openerp/commit/9b18432d300176f5fbea5e9d0687f7f6d753c363))
* Docker containerization with PostgreSQL and UI improvements ([de9f562](https://github.com/hansjlachmann/openerp/commit/de9f56258cba6ae88bfca91b32ee20382cbef3b6))
* Docker containerization with PostgreSQL and UI improvements ([cde09e8](https://github.com/hansjlachmann/openerp/commit/cde09e89ddb92c9a01f62e2d2a4a8c11d0d71cae))
* Front-end UI ([c2f1f59](https://github.com/hansjlachmann/openerp/commit/c2f1f594c8e46bd2ae048b2c156bb34bb9fb2849))
* genereric error messages from foundation layer ([41cc54f](https://github.com/hansjlachmann/openerp/commit/41cc54f64ca2e0e8960551dde42fd87810cef624))
* implement user-assignable menu system ([b1acd0a](https://github.com/hansjlachmann/openerp/commit/b1acd0a538b610d4176dba258ebd24a235138147))
* improve Customer Card modal UX and Edit button functionality ([1d3c6b3](https://github.com/hansjlachmann/openerp/commit/1d3c6b3ecf90ef3bc34f7c0e7929556126f1242a))
* improve Customer Card UX and fix button states ([513fdaa](https://github.com/hansjlachmann/openerp/commit/513fdaa057b38f3a0f45591c3fb09aeab60cfb1d))
* Redesign FilterPane with classic ERP-style Views ([1de6219](https://github.com/hansjlachmann/openerp/commit/1de621939fd504cc9db068edc9fc62748011ff89))
* Reorganize page header buttons layout ([e6bcb03](https://github.com/hansjlachmann/openerp/commit/e6bcb03594931e3bd95d8b4d3aed6ffa3a0612d7))
* show lookup dropdowns in non-edit mode on card pages ([019d4a7](https://github.com/hansjlachmann/openerp/commit/019d4a70c8753c47e5dc8dbccf2f8ddf97b1074d))
* smart auto-save with change detection and UI improvements ([4687664](https://github.com/hansjlachmann/openerp/commit/468766481878baf1bc6ddabf6ff5b364d2fbed6d))
* UI components, multi-language support, and editable list improvements ([#17](https://github.com/hansjlachmann/openerp/issues/17)) ([0391eb4](https://github.com/hansjlachmann/openerp/commit/0391eb4ffaf046b0e24889167be19804b9276943))


### Bug Fixes

* add empty ID validation to ModifyRecord and DeleteRecord ([e2d9492](https://github.com/hansjlachmann/openerp/commit/e2d9492f435417605e83299154549874287e7a68))
* add empty ID validation to ModifyRecord and DeleteRecord ([9b1a341](https://github.com/hansjlachmann/openerp/commit/9b1a341b8ac7652559a98629e7facf8476bbeb06))
* Add ensureTableExists to create tables on-demand from metadata ([8241e27](https://github.com/hansjlachmann/openerp/commit/8241e277b1caf96809f67adb345029bd2642d651))
* Add missing caption for Payment Terms 'active' field ([7c48f2b](https://github.com/hansjlachmann/openerp/commit/7c48f2ba8acfdd1c5b6945b2c7b337594e3c9856))
* Add missing fyne.io/fyne/v2 import for GUI ([be2ffe6](https://github.com/hansjlachmann/openerp/commit/be2ffe63eea44fb781cb13a60ecb1f7693193ba8))
* add table_relation to language field in User card page ([f8704c9](https://github.com/hansjlachmann/openerp/commit/f8704c9bade4dcd7f32572048b742b04560db604))
* correct Norwegian translation for preferences ([824ae69](https://github.com/hansjlachmann/openerp/commit/824ae691e1706167c4f0c61834d4edacf4d0b26d))
* correct TypeScript type for lookup data in getTableOptionsAndLookups ([582be24](https://github.com/hansjlachmann/openerp/commit/582be245febca6ccbe5a09203b43f72fd42b6bbd))
* Create FieldDefinition table in OpenDatabase for backward compatibility ([abd18c6](https://github.com/hansjlachmann/openerp/commit/abd18c6daa3f4e1ce7355b9604c4797bb5332bf0))
* CreateTable now inserts marker record so table shows in ListTables ([2cffb52](https://github.com/hansjlachmann/openerp/commit/2cffb52bd57923cceb61c83156c443166d2e6dd7))
* day and night mode ([6fe82e6](https://github.com/hansjlachmann/openerp/commit/6fe82e604a65259e06029dd58c021a5dbdd72511))
* focus first input when opening card page modal ([000475a](https://github.com/hansjlachmann/openerp/commit/000475a97d8daab8106c91b653e3d87b9c20fdf6))
* handle NULL database values and improve new record detection ([4b0da75](https://github.com/hansjlachmann/openerp/commit/4b0da7528e201b9df7542257338c85ade289d218))
* list page border now ends at last record ([7f523a5](https://github.com/hansjlachmann/openerp/commit/7f523a5c26c347740875f7922077217643161dd8))
* make list page column headers sticky when scrolling ([9b538b7](https://github.com/hansjlachmann/openerp/commit/9b538b75e3b163f0f289220ece2bfbc65ad11955))
* make list page column headers sticky when scrolling ([c351a47](https://github.com/hansjlachmann/openerp/commit/c351a47a486af7a2c4bb8cda948acdce4ef0e8e9))
* make list page column headers sticky when scrolling ([0181231](https://github.com/hansjlachmann/openerp/commit/01812316f227a3c172bdfa4fc5cbf84e7bfa46a9))
* modal card close functionality and keyboard shortcut handling ([800db71](https://github.com/hansjlachmann/openerp/commit/800db71e0aca0c0d88755fb063200cdb55475947))
* modal card close functionality and keyboard shortcut handling ([eb0f5e7](https://github.com/hansjlachmann/openerp/commit/eb0f5e7edbb66fbdad153615e9f842a63d1550f1))
* Move authentication check to layout load function ([76f185d](https://github.com/hansjlachmann/openerp/commit/76f185deb584045ab8b14eabc976fba6fec60922))
* prevent creating records with empty primary key ([fa3b505](https://github.com/hansjlachmann/openerp/commit/fa3b505fd318e0a96b706e4e12cacf127186da42))
* prevent creating records with empty primary key ([2291e06](https://github.com/hansjlachmann/openerp/commit/2291e061a52a45016b3a589c1da11fb1e1d29ba9))
* reduce row height for option dropdowns in list page ([ac2335f](https://github.com/hansjlachmann/openerp/commit/ac2335f9e26cb606f23b04b7ecc9fa2e663035fe))
* register Language table in table registry ([b389fa6](https://github.com/hansjlachmann/openerp/commit/b389fa6154831ef13ec688a35e7285c547ac3c4e))
* Remove nested go.mod files and update import paths ([eff6084](https://github.com/hansjlachmann/openerp/commit/eff60847667ecb0edd4406c51e47a22b32b5694a))
* resolve go vet errors in backend code ([3baf769](https://github.com/hansjlachmann/openerp/commit/3baf769d3ffaae7c7a9a8f6f4d7da9d286cf0e1d))
* resolve golangci-lint errors ([0d3ce8e](https://github.com/hansjlachmann/openerp/commit/0d3ce8e535d49a7302ea19f885d2b9472200b0aa))
* resolve remaining a11y warnings ([f50e8f8](https://github.com/hansjlachmann/openerp/commit/f50e8f895e41f7c4b48a5a0baad6c70637cbafba))
* resolve Svelte 5 props_invalid_value error for lookup dropdowns ([9c1ca22](https://github.com/hansjlachmann/openerp/commit/9c1ca221c735e0cbf1673b4aee660a9bb2e023ab))
* resolve Svelte 5 warnings ([7189359](https://github.com/hansjlachmann/openerp/commit/71893592eaaaa4643d8d7e078076b2817b7da1ed))
* resolve svelte-check warnings ([b5052f0](https://github.com/hansjlachmann/openerp/commit/b5052f01340adf58a0fb48304549b0285f36c2df))
* resolve TypeScript errors in frontend build ([3e5345f](https://github.com/hansjlachmann/openerp/commit/3e5345f52761d139ac2dfe96c44c2c3777220b16))
* update gitignore ([f6df9f6](https://github.com/hansjlachmann/openerp/commit/f6df9f6f1344ae6057a9d1e49c9b42ec6190d3ee))
* use generic action button captions ([55ad1ce](https://github.com/hansjlachmann/openerp/commit/55ad1cef1c2a70a24f24c4061a64089d800cd91d))
* use getRecordId helper for delete and row click in PageRenderer ([4cd7a81](https://github.com/hansjlachmann/openerp/commit/4cd7a81fe9acbd42fb7908331df476cff38ae4a9))


### Code Refactoring

* Auto-initialize tables and remove Object Designer ([2d21d31](https://github.com/hansjlachmann/openerp/commit/2d21d31af1b2906b5acd691e93d420ea7ee6b45f))
* consolidate confirmation modal to shared store ([a7d76bd](https://github.com/hansjlachmann/openerp/commit/a7d76bd9eb352a09e456dea3c067971c5aff9326))
* consolidate duplicate code across frontend ([9a83a16](https://github.com/hansjlachmann/openerp/commit/9a83a168890c0c384ccb07757500e75d101936a1))
* consolidate duplicate code and improve code organization ([8e5b130](https://github.com/hansjlachmann/openerp/commit/8e5b13087aa5797b8c0f826a288351bb1b59e30d))
* create centralized localStorage utility ([ae3bfa6](https://github.com/hansjlachmann/openerp/commit/ae3bfa6e8502cccd8ee7462968d1fa1f9efb3b79))
* Extract duplicate code into reusable utilities and components ([fdcff3b](https://github.com/hansjlachmann/openerp/commit/fdcff3bc83c181343592e8edba334bbc8a9e8e6a))
* extract shared utilities for record handling and API helpers ([62fb453](https://github.com/hansjlachmann/openerp/commit/62fb4530e52ab6d52dc894cccb10326ef058843d))
* extract visibility logic and remove dead code ([d7812be](https://github.com/hansjlachmann/openerp/commit/d7812bec1b2f7867dae7f4023c98b62bdaea10a3))
* implement generic Table interface for API handlers ([fe4bf7f](https://github.com/hansjlachmann/openerp/commit/fe4bf7f0491a7033e47cbcc9f0aa55c52aead951))
* make frontend fully generic using primary_key from page definitions ([c50fc81](https://github.com/hansjlachmann/openerp/commit/c50fc81499e0ad98c0a86d41a7ce70cb37e8003b))
* Move pages folder to business logic layer ([b839a2e](https://github.com/hansjlachmann/openerp/commit/b839a2e5e2e26eccf980d608ef9b3abf1f28be31))
* remove legacy hardcoded customer code ([b7048cd](https://github.com/hansjlachmann/openerp/commit/b7048cdb53b8290a1467478dcd11ae284a70ae56))
* separate generated table code from manual business logic ([35b5b2b](https://github.com/hansjlachmann/openerp/commit/35b5b2b6a5fc83b708ab4066ffa7f8a0ec939461))
* translate all hardcoded API error messages ([f61fbf8](https://github.com/hansjlachmann/openerp/commit/f61fbf8c6c025617d42f7a9b067e9eb0d7294815))
* translate all hardcoded API error messages ([eff5574](https://github.com/hansjlachmann/openerp/commit/eff557482ff0a3d9e079e60e811548be584754ee))


### CI/CD

* add Docker build and push to GitHub Container Registry ([75d4999](https://github.com/hansjlachmann/openerp/commit/75d4999c82f8ec65ca675eb89ddc1afc86705b65))
* add GitHub Actions build and lint workflow ([6585658](https://github.com/hansjlachmann/openerp/commit/658565825396651ba7ac25d698a3746cc2ac7113))
* add Go test step with race detection and coverage ([5b6e03c](https://github.com/hansjlachmann/openerp/commit/5b6e03c37bd06c69e3b768b66a59efd60c39d616))
* add golangci-lint step ([8fd3ca7](https://github.com/hansjlachmann/openerp/commit/8fd3ca780766acce15420d949680080bc62fbaba))
* add release-please for automated releases ([ffd6cdd](https://github.com/hansjlachmann/openerp/commit/ffd6cdd07f8c85b7633b7f9f19d7400831ce140d))
* merge build and release workflows into single file ([49d8819](https://github.com/hansjlachmann/openerp/commit/49d8819266397e4afd1352e2dee5bafb5821e0c4))
