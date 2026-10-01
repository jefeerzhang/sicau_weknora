import json
import time
import uuid
import urllib.error
import urllib.parse
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
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
    body = "\r\n".join(lines).encode("utf-8")
    return body, f"multipart/form-data; boundary={boundary}"


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
    """Extract share-link plaintext token from invite_url (?token=...)."""
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

    admin_password = "admin123"
    code, login = req("POST", "/auth/login", {"email": "admin@admin.com", "password": admin_password})
    if code != 200 or not login.get("token"):
        for candidate in ("AdminPass9y", "AdminPass9x"):
            admin_password = candidate
            code, login = req(
                "POST", "/auth/login", {"email": "admin@admin.com", "password": admin_password}
            )
            if code == 200 and login.get("token"):
                break
    if code != 200 or not login.get("token"):
        print("FINDINGS", ["FAIL admin login"])
        return
    findings.append("PASS admin login")
    token = login["token"]
    user = login.get("user") or {}
    print(
        "admin",
        {
            "is_system_admin": user.get("is_system_admin"),
            "must_change_password": user.get("must_change_password"),
            "platform_identity": user.get("platform_identity"),
            "is_teacher": user.get("is_teacher"),
        },
    )

    if user.get("must_change_password"):
        new_password = "AdminPass9y"
        code, ch = req(
            "POST",
            "/auth/change-password",
            {"old_password": admin_password, "new_password": new_password},
            token=token,
        )
        if code == 200:
            code, login = req("POST", "/auth/login", {"email": "admin@admin.com", "password": new_password})
            if code == 200 and login.get("token"):
                token = login["token"]
                admin_password = new_password
                findings.append("PASS must_change_password")
            else:
                findings.append("FAIL must_change_password re-login")
                print("FINDINGS", findings)
                return
        else:
            findings.append(f"WARN must_change_password change failed ({code}); continue with current password")
            # Flag may remain true until a successful distinct rotation.

    code, _ = req("GET", "/auth/me", token=token)
    findings.append("PASS /auth/me" if code == 200 else f"FAIL /auth/me {code}")

    # SuperAdmin creates course space (teacher flow proxy)
    code, tenant_resp = req(
        "POST",
        "/tenants",
        {"name": "QA课程空间", "description": "sync qa smoke"},
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

    # create teacher account then appoint (idempotent-ish; reuse if exists)
    suffix = str(int(time.time()))
    teacher_email = f"teacher.qa.{suffix}@example.com"
    teacher_password = "TeacherPass9"
    code, _ = req(
        "POST",
        "/system/admin/users/create",
        {
            "email": teacher_email,
            "username": f"teacherqa{suffix}",
            "password": teacher_password,
        },
        token=token,
    )
    findings.append("PASS create teacher user" if code in (200, 201) else f"WARN create teacher user {code}")
    code, _ = req(
        "POST",
        "/system/admin/teachers/appoint",
        {"email": teacher_email},
        token=token,
    )
    findings.append("PASS appoint teacher" if code == 200 else f"WARN appoint teacher {code}")

    # student onboarding uses share-link invite (not email invitation to
    # already-registered users).
    code, inv = req(
        "POST",
        f"/tenants/{tid}/invite-links",
        {"role": "viewer", "message": "QA smoke share link"},
        token=token,
        extra_headers=th,
    )
    invite_token = dig_invite_token(inv)
    findings.append("PASS create invite-link" if code in (200, 201) else f"FAIL invite-link {code}")
    if invite_token:
        findings.append("PASS invite token present")
    else:
        findings.append("FAIL invite token missing in invite_url")

    # announcement requires multipart form fields
    code, _ = req(
        "POST",
        "/announcements",
        token=token,
        extra_headers=th,
        form={"title": "QA公告", "content": "冒烟测试公告正文"},
    )
    findings.append("PASS create announcement" if code in (200, 201) else f"WARN announcement {code}")

    code, _ = req("GET", f"/tenants/{tid}/members", token=token, extra_headers=th)
    findings.append("PASS admin list members" if code == 200 else f"WARN admin members {code}")

    # student register + sealed surfaces
    student_email = f"student.qa.{suffix}@example.com"
    if invite_token:
        code, reg = req(
            "POST",
            "/auth/register-by-invite",
            {
                "email": student_email,
                "username": f"studentqa{suffix}",
                "password": "StudentPass9",
                "token": invite_token,
            },
        )
        findings.append("PASS student register-by-invite" if code in (200, 201) else f"WARN student register {code}")
        stoken = None
        if code in (200, 201) and reg.get("token"):
            stoken = reg["token"]
            findings.append("PASS student auto-login from register")
        else:
            code, st = req(
                "POST",
                "/auth/login",
                {"email": student_email, "password": "StudentPass9"},
            )
            if code == 200 and st.get("token"):
                stoken = st["token"]
                findings.append("PASS student login")
            else:
                findings.append("WARN student login failed")
        if stoken:
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
                else f"WARN student env-vars {code}"
            )
            code, _ = req("GET", "/me/notes", token=stoken, extra_headers=th)
            findings.append("PASS student notes readable" if code == 200 else f"WARN student notes {code}")
            code, _ = req("GET", "/announcements", token=stoken, extra_headers=th)
            findings.append(
                "PASS student announcements readable" if code == 200 else f"WARN student announcements {code}"
            )
            code, _ = req("GET", "/agents", token=stoken, extra_headers=th)
            findings.append(f"INFO student agents list {code}")
    else:
        findings.append("SKIP student path (no invite token)")

    print("==== FINDINGS ====")
    for f in findings:
        print(f)


if __name__ == "__main__":
    main()
