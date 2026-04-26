# Mission 3 Baseline — search_fuzzy before RRF time-decay

- Measured: 2026-04-26T05:36:50Z
- Commit: `c81b9cff141f`
- Corpus size: 1004 memories

## Summary

| Category | n | Top-1 | Top-3 | Top-10 |
|---|---:|---:|---:|---:|
| pure_recency | 5 | 0% | 0% | 20% |
| pure_relevance | 5 | 40% | 40% | 60% |
| mixed | 5 | 20% | 60% | 80% |
| adversarial | 5 | 20% | 80% | 80% |
| overall | 20 | 20% | 45% | 60% |

**Headline finding:** pure_recency top-1 hit rate is 0% on today's corpus; Mission 3 should target this gap without collapsing old high-signal memories.

## Per-query results

| id | category | rank | top-1 | top-3 | |
|---|---|---:|---|---|---|
| rec-01 | pure_recency | null | no | no | |
| rec-02 | pure_recency | null | no | no | |
| rec-03 | pure_recency | null | no | no | |
| rec-04 | pure_recency | 4 | no | no | |
| rec-05 | pure_recency | null | no | no | |
| rel-01 | pure_relevance | 1 | yes | yes | |
| rel-02 | pure_relevance | null | no | no | |
| rel-03 | pure_relevance | 1 | yes | yes | |
| rel-04 | pure_relevance | null | no | no | |
| rel-05 | pure_relevance | 4 | no | no | |
| mix-01 | mixed | 2 | no | yes | |
| mix-02 | mixed | 3 | no | yes | |
| mix-03 | mixed | 1 | yes | yes | |
| mix-04 | mixed | 4 | no | no | |
| mix-05 | mixed | null | no | no | |
| adv-01 | adversarial | null | no | no | |
| adv-02 | adversarial | 2 | no | yes | |
| adv-03 | adversarial | 2 | no | yes | |
| adv-04 | adversarial | 2 | no | yes | |
| adv-05 | adversarial | 1 | yes | yes | |

## Notes on outliers

- `rec-01` missed the top 10 entirely; expected winner `994` was absent from returned IDs [902, 905, 870, 889, 888, 872, 908, 773, 617, 911].
- `rec-02` missed the top 10 entirely; expected winner `911` was absent from returned IDs [889, 905, 888, 611, 872, 613, 598, 870, 617, 887].
- `rec-03` missed the top 10 entirely; expected winner `997` was absent from returned IDs [918, 948, 771, 954, 603, 924, 577, 949, 340, 605].
- `rec-04` ranked 4; this is outside the top 3 and is a likely tuning target.
- `rec-05` missed the top 10 entirely; expected winner `988` was absent from returned IDs [276, 408, 928, 621, 234, 204, 986, 357, 100, 218].
- `rel-02` missed the top 10 entirely; expected winner `918` was absent from returned IDs [683, 634, 765, 709, 182, 929, 648, 978, 406, 392].
- `rel-04` missed the top 10 entirely; expected winner `922` was absent from returned IDs [988, 926, 987, 989, 986, 983, 993, 991, 982, 575].
- `rel-05` ranked 4; this is outside the top 3 and is a likely tuning target.
- `mix-04` ranked 4; this is outside the top 3 and is a likely tuning target.
- `mix-05` missed the top 10 entirely; expected winner `988` was absent from returned IDs [956, 928, 922, 918, 973, 218, 286, 417, 252, 294].
- `adv-01` missed the top 10 entirely; expected winner `7` was absent from returned IDs [276, 408, 234, 405, 357, 621, 347, 387, 154, 986].
