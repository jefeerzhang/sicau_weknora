#!/usr/bin/env python3
"""Route 2 upgrade smoke against a cloned main DB on APP_PORT (default 8081)."""

import json
import os
import time
import uuid
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("WEKNORA_QA_BASE", "http://127.0.0.1:8081/api/v1")
EMAIL = os.environ.get("WEKNORA_QA_EMAIL", "494808401@qq.com")
PASSWORD = os.environ.get("WEKNORA_QA_PASSWORD", "UpgradeQA9x")
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def encode_multipart(fields):
    boundary = f"----WeKnoraQA{uuid.uuid4().hex}"
    lines = []
    for name, value in fields.items():
        lines.append(f"--{boundary}")
        lines.append(f'Content-Disposition: form-data; name="{name}"')
        lines.append("")
        lines.append(str(value))
    lines.append(f"--{boundary}--")
    lines.append("")
    return "\r\n".join(lines).encode("utf-8"), f"multipart/form-data; boundary={boundary}"


def req(method, path, body=None, token=None, extra_headers=None, form=None):
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if extra_headers:
        headers.update(extra_headers)
    data = None
    if form is not None:
        data, content_type = encode_multipart(form)
        headers["Content-Type"] = content_type
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with opener.open(request, timeout=45) as resp:
            raw = resp.read().decode()
            code = resp.status
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        code = e.code
    try:
        payload = json.loads(raw) if raw else {}
    except Exception:
        payload = {"_raw": raw[:500]}
    msg = payload.get("message") or payload.get("error") or ""
    if isinstance(msg, dict):
        msg = json.dumps(msg, ensure_ascii=False)[:200]
    print(f"{method} {path} -> {code} success={payload.get('success')} msg={msg}")
    return code, payload


def dig_id(obj):
    if not isinstance(obj, dict):
        return None
    for k in ("id", "tenant_id"):
        if obj.get(k) is not None:
            return obj.get(k)
    data = obj.get("data")
    if isinstance(data, dict):
        for k in ("id", "tenant_id"):
            if data.get(k) is not None:
                return data.get(k)
    return None


def dig_invite_token(obj):
    if not isinstance(obj, dict):
        return None
    url = obj.get("invite_url")
    if isinstance(url, str) and "token=" in url:
        q = urllib.parse.parse_qs(urllib.parse.urlparse(url).query)
        if q.get("token"):
            return q["token"][0]
    data = obj.get("data")
    if isinstance(data, dict):
        return dig_invite_token(data)
    return None


def main():
    findings = []
    code, login = req("POST", "/auth/login", {"email": EMAIL, "password": PASSWORD})
    if code != 200 or not login.get("token"):
        print("FINDINGS", ["FAIL upgrade admin login"])
        return
    findings.append("PASS upgrade admin login")
    token = login["token"]

    code, cfg = req("GET", "/auth/config")
    data = cfg.get("data") if isinstance(cfg.get("data"), dict) else cfg
    findings.append(f"INFO registration_mode={data.get('registration_mode')}")

    suffix = str(int(time.time()))
    code, tenant_resp = req(
        "POST",
        "/tenants",
        {"name": f"升级QA空间{suffix}", "description": "route2 upgrade smoke"},
        token=token,
    )
    tid = dig_id(tenant_resp)
    if not tid:
        code, listed = req("GET", "/tenants", token=token)
        items = listed.get("data") or listed.get("tenants") or []
        if isinstance(items, dict):
            items = items.get("items") or items.get("data") or []
        if isinstance(items, list) and items:
            tid = items[0].get("id")
    if not tid:
        findings.append("FAIL create/list tenant")
        print("FINDINGS", findings)
        return
    findings.append(f"PASS tenant ready id={tid}")
    th = {"X-Tenant-ID": str(tid)}

    teacher_email = f"teacher.up.{suffix}@example.com"
    code, _ = req(
        "POST",
        "/system/admin/users/create",
        {
            "email": teacher_email,
            "username": f"teacherup{suffix}",
            "password": "TeacherPass9",
        },
        token=token,
    )
    findings.append("PASS create teacher" if code in (200, 201) else f"WARN create teacher {code}")
    code, _ = req(
        "POST",
        "/system/admin/teachers/appoint",
        {"email": teacher_email},
        token=token,
    )
    findings.append("PASS appoint teacher" if code == 200 else f"WARN appoint teacher {code}")

    code, inv = req(
        "POST",
        f"/tenants/{tid}/invite-links",
        {"role": "viewer", "message": "upgrade smoke"},
        token=token,
        extra_headers=th,
    )
    invite_token = dig_invite_token(inv)
    findings.append(
        "PASS invite-link" if code in (200, 201) and invite_token else f"FAIL invite-link {code}"
    )

    code, _ = req(
        "POST",
        "/announcements",
        token=token,
        extra_headers=th,
        form={"title": "升级公告", "content": "route2"},
    )
    findings.append("PASS announcement" if code in (200, 201) else f"WARN announcement {code}")

    code, _ = req("GET", f"/tenants/{tid}/members", token=token, extra_headers=th)
    findings.append("PASS admin members" if code == 200 else f"WARN members {code}")

    student_email = f"student.up.{suffix}@example.com"
    if invite_token:
        code, reg = req(
            "POST",
            "/auth/register-by-invite",
            {
                "email": student_email,
                "username": f"studentup{suffix}",
                "password": "StudentPass9",
                "token": invite_token,
            },
        )
        findings.append(
            "PASS student register" if code in (200, 201) else f"FAIL student register {code}"
        )
        stoken = reg.get("token") if code in (200, 201) else None
        if stoken:
            findings.append("PASS student auto-login")
            code, _ = req("GET", f"/tenants/{tid}/members", token=stoken, extra_headers=th)
            findings.append(
                "PASS student members sealed"
                if code in (401, 403)
                else f"WARN student members {code}"
            )
            code, _ = req("GET", "/me/env-vars", token=stoken, extra_headers=th)
            findings.append(
                "PASS student env-vars sealed"
                if code in (401, 403)
                else f"WARN env-vars {code}"
            )
            code, _ = req("GET", "/me/notes", token=stoken, extra_headers=th)
            findings.append("PASS student notes" if code == 200 else f"WARN notes {code}")
            code, _ = req("GET", "/announcements", token=stoken, extra_headers=th)
            findings.append(
                "PASS student announcements" if code == 200 else f"WARN announcements {code}"
            )

    print("==== ROUTE2 FINDINGS ====")
    for f in findings:
        print(f)


if __name__ == "__main__":
    main()
