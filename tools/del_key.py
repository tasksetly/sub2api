import httpx
headers = {
  "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo3MzksImVtYWlsIjoiMjUwMTczMzk2QHFxLmNvbSIsInJvbGUiOiJ1c2VyIiwidG9rZW5fdmVyc2lvbiI6MTA3MjUwMjcxODEwNjMxNDczNSwic2lkIjoiOTk1MmU5MThhMzI1OGUzZjliZmFhMzk5NTc2NWEwYjEiLCJibmQiOiI1MDk5NTZmZDE1MWYzYTE2ODdjZGMxNTZjODI3MzIwNiIsImV4cCI6MTc4NDkwMTM5OSwibmJmIjoxNzg0ODE0OTk5LCJpYXQiOjE3ODQ4MTQ5OTl9.I08aESMwFa6Qx4YRxIUCc4pFH5s83LsBBlDn5Y03cy8"
}

del_url = "https://aihub.top/api/v1/keys/9344"
ids = [
    8681,
    8680,
    8679,
    8678,
    8677,
    8676,
    8675,
    8674,
    8673,
    8672,
    8671,
    8670,
    8669,
    8668,
    8667,
    8637,
    7893,
    5066,
    4272,
    4254,
    4194,
    2917,
    2437,
    467
]
with httpx.Client() as h:
    for i in ids:
        url = f"https://aihub.top/api/v1/keys/{i}"
        r = h.delete(url,headers=headers)
        print(r.json())