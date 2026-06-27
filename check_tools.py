import json

d = json.load(open(r'e:\Dev\Go\locolm\test_resp.json'))
tools = d['result']['tools']
print(f'Total tools: {len(tools)}')
for t in tools:
    has_os = 'outputSchema' in t
    print(f'  {t["name"]}: outputSchema={has_os}')
