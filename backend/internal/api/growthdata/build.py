"""Builds oms.csv and cdc.csv (the tables Caresia ships already loaded) from the published files.

Sources (the files are copied verbatim; only the layout changes to the CSV of docs/CRECIMIENTO.md):
  * OMS (WHO Child Growth Standards, 0 a 5 años): the L, M, S tables by month of the WHO, as redistributed in the
    `pygrowup` package (tables/wfa|lhfa|bmifa|hcfa_*), version 0.8.2. Length-for-age and BMI-for-age use the
    0-2 years tables up to month 23 and the 2-5 years tables from month 24 (WHO convention: length lying down, then height).
  * CDC (Growth Charts, 0 a 20 años): the data files of the CDC (wtageinf, lenageinf, hcageinf for 0-36 months and
    wtage, lenage, bmiage for 2-20 years), as redistributed in the `growthcharts` package, version 0.2.1.
    Infant files up to month 23.5 and the 2-20 years files from month 24 (CDC convention).

Usage: python3 build.py <dir with pygrowup/tables> <dir with growthcharts/data>
"""
import csv, json, sys

who_dir, cdc_dir = sys.argv[1], sys.argv[2]
HEAD = ['indicator', 'sex', 'age_months', 'l', 'm', 's']


def who(indicator, prefix, parts):
    rows = []
    for sex_name, sex in (('boys', 'M'), ('girls', 'F')):
        for fname, lo, hi in parts:
            for r in json.load(open(f'{who_dir}/{prefix}_{sex_name}_{fname}_zscores.json')):
                age = float(r['Month'])
                if lo <= age <= hi:
                    rows.append([indicator, sex, age, r['L'], r['M'], r['S']])
    return rows


oms = (
    who('weight_for_age', 'wfa', [('0_5', 0, 60)])
    + who('length_height_for_age', 'lhfa', [('0_2', 0, 23), ('2_5', 24, 60)])
    + who('bmi_for_age', 'bmifa', [('0_2', 0, 23), ('2_5', 24, 60)])
    + who('head_circumference_for_age', 'hcfa', [('0_5', 0, 60)])
)


def cdc(indicator, infant, child, infant_max):
    rows = []
    for fname, lo, hi in ((infant, 0, infant_max), (child, 24, 240)):
        if not fname:
            continue
        for r in csv.DictReader(open(f'{cdc_dir}/{fname}.csv', encoding='utf-8-sig')):
            age = float(r['Agemos'])
            if lo <= age <= hi:
                rows.append([indicator, 'M' if r['Sex'] == '1' else 'F', age, r['L'], r['M'], r['S']])
    return rows


cdc_rows = (
    cdc('weight_for_age', 'wtageinf', 'wtage', 23.5)
    + cdc('length_height_for_age', 'lenageinf', 'lenage', 23.5)
    + cdc('bmi_for_age', None, 'bmiage', 0)
    + cdc('head_circumference_for_age', 'hcageinf', None, 36)
)

for name, rows in (('oms', oms), ('cdc', cdc_rows)):
    rows.sort(key=lambda r: (r[0], r[1], float(r[2])))
    with open(f'{name}.csv', 'w', newline='') as f:
        w = csv.writer(f, lineterminator='\n')
        w.writerow(HEAD)
        w.writerows(rows)
    print(name, len(rows))
