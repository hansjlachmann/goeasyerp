# Loyalty Points (test app)

A small extension app the base application's tests build with: it is merged into the base
definitions in CI (`scripts/check-extension-fixture.sh`), so a change to the base or to the
extension tools that would break extension apps fails there. It is not shipped and not an
example to copy — the example app lives in a repository of its own (see `EXTENSIONS.md`).

What it adds: a field on Customer (`lp_loyalty_points`, `lp_tier_code`) shown on the Customer
Card and the Customer List, a table and list page of its own (Loyalty Tiers, 50000), a button on
both pages and an entry in the Sales menu.
