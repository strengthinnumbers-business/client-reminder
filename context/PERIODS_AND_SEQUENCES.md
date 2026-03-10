# Periods and Sequences

- Each client is assigned a region (typically a province or a territory) which informs which days are considered statutory holidays.

- This app does only trigger email sending on business days (Monday to Friday) that are not considered to be statutory holidays in the client's region.

- The first Monday of a period is the first candidate for the day with a "sequence day offset" of 0. If it falls on a statutory holiday in the client's region, then the next business day that is not a statutory holiday is used for "sequence day offset" of 0.

- Sequence Day Offsets are only counted on business days that are not statutory holidays in the client's region.

- This app uses the https://raw.githubusercontent.com/pcraig3/hols/refs/heads/main/reference/Canada-Holidays-API.v1.yaml OpenAPI spec for the https://canada-holidays.ca/api API to find out which regions (provinces or territories) observe which statutory holidays. Please use this API with plenty of caching time, a default of 1 lookup per month is sufficient. 
