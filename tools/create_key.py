import httpx


url = "https://aihub.top/api/v1/public/monitor/summary?timezone=Asia%2FShanghai"
headers = {
  "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo3MzksImVtYWlsIjoiMjUwMTczMzk2QHFxLmNvbSIsInJvbGUiOiJ1c2VyIiwidG9rZW5fdmVyc2lvbiI6MTA3MjUwMjcxODEwNjMxNDczNSwic2lkIjoiOTk1MmU5MThhMzI1OGUzZjliZmFhMzk5NTc2NWEwYjEiLCJibmQiOiI1MDk5NTZmZDE1MWYzYTE2ODdjZGMxNTZjODI3MzIwNiIsImV4cCI6MTc4NDkwMTM5OSwibmJmIjoxNzg0ODE0OTk5LCJpYXQiOjE3ODQ4MTQ5OTl9.I08aESMwFa6Qx4YRxIUCc4pFH5s83LsBBlDn5Y03cy8"
}

create_aihub_key_url = "https://aihub.top/api/v1/keys"


task_create_api_url = "https://ai.tasksetly.com/api/v1/admin/accounts"

login_url = "https://ai.tasksetly.com/api/v1/auth/login"
login_data = {
    "email": "admin@sub2api.local",
    "password": "c5d142bd369d6f10f79ca0a2ba4b0ea4"
}


# 获取aihub的渠道分组，只要存活大于99%的就创建
with httpx.Client() as h:
    r = h.post(login_url,json=login_data)
    data = r.json()
    access_token = data.get("data").get("access_token")
    task_headers = {
        "Authorization":f"Bearer {access_token}"
    }


    r = h.get(url,headers=headers)
    r_json = r.json()
    for api in r_json.get("apis"):
        available = api.get("available")
        if available:
            key_name = f"{api.get('planType')}_{api.get('id')}"
            group_id = api.get('group_id')
            data = {
                "name":key_name,
                "group_id":group_id,
                "max_rate_multiplier":0
            }
            r2 = h.post(create_aihub_key_url,headers=headers,json=data)
            key = r2.json().get('data').get('key')
            print(key)
            data = {
                "name": key_name,
                "supplier": "aihub",
                "notes": key_name,
                "platform": "openai",
                "type": "apikey",
                "credentials": {
                    "base_url": "https://aihub.top",
                    "api_key": key,
                    "model_mapping": {
                        "gpt-5.4": "gpt-5.4",
                        "gpt-5.5": "gpt-5.5",
                        "gpt-5.6-luna": "gpt-5.6-luna",
                        "gpt-5.6-sol": "gpt-5.6-sol",
                        "gpt-5.6-terra": "gpt-5.6-terra"
                    },
                    "pool_mode": True,
                    "pool_mode_retry_count": 3,
                    "temp_unschedulable_enabled": True,
                    "temp_unschedulable_rules": [
                        {
                            "error_code": 503,
                            "keywords": [
                                "unavailable",
                                "maintenance"
                            ],
                            "duration_minutes": 30,
                            "description": "服务不可用 - 暂停 30 分钟"
                        },
                        {
                            "error_code": 429,
                            "keywords": [
                                "rate limit",
                                "too many requests"
                            ],
                            "duration_minutes": 5,
                            "description": "触发限流 - 暂停 10 分钟"
                        },
                        {
                            "error_code": 529,
                            "keywords": [
                                "overloaded",
                                "too many"
                            ],
                            "duration_minutes": 60,
                            "description": "服务过载 - 暂停 60 分钟"
                        }
                    ]
                },
                "proxy_id": None,
                "concurrency": 10,
                "load_factor": None,
                "priority": 1,
                "rate_multiplier": 1,
                "group_ids": [
                    2 if 'pro' not in key_name else 4
                ],
                "expires_at": None,
                "extra": {
                    "openai_apikey_responses_websockets_v2_mode": "off",
                    "openai_apikey_responses_websockets_v2_enabled": False,
                    "openai_long_context_billing_enabled": False
                },
                "upstream_billing_probe_enabled": True,
                "auto_pause_on_expired": True
            }
            
            r3 = h.post(task_create_api_url,headers=task_headers,json=data)
            