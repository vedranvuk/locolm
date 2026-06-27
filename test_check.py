import json

with open(r'e:\Dev\Go\locolm\test_resp.json') as f:
    raw = f.read()

# Check if valid JSON
try:
    d = json.loads(raw)
    print("Valid JSON: YES")
except json.JSONDecodeError as e:
    print(f"Valid JSON: NO - {e}")
    print(f"First 500 chars: {raw[:500]}")
    exit(1)

if 'result' in d and 'tools' in d['result']:
    tools = d['result']['tools']
    print(f"Total tools: {len(tools)}")
    for t in tools:
        has_os = 'outputSchema' in t
        os_val = t.get('outputSchema', 'MISSING')
        if has_os and os_val == '':
            print(f"  {t['name']}: outputSchema is EMPTY STRING - BUG!")
        elif has_os:
            print(f"  {t['name']}: has outputSchema ({len(str(os_val))} chars)")
        else:
            print(f"  {t['name']}: no outputSchema (omitted)")
else:
    print(f"No tools in response. Keys: {list(d.keys())}")
    print(f"Full response: {raw[:500]}")
