FROM ghcr.io/open-webui/open-webui:latest

RUN python - <<'PY'
from pathlib import Path
import sys

path = Path("/app/backend/open_webui/utils/auth.py")
source = path.read_text()

if "PAI_COOKIE_FALLBACK_PATCH" in source:
    print("auth.py is already patched")
    sys.exit(0)

old_token_block = """    token = None

    if auth_token is not None:
        token = auth_token.credentials

    if token is None and "token" in request.cookies:
        token = request.cookies.get("token")

    # Fallback to request.state.token (set by middleware, e.g. for x-api-key)
    if token is None and hasattr(request.state, "token") and request.state.token:
        token = request.state.token.credentials

    if token is None:
        raise HTTPException(status_code=401, detail="Not authenticated")
"""

new_token_block = """    # PAI_COOKIE_FALLBACK_PATCH: preserve valid cookie session when Authorization is stale.
    header_token = auth_token.credentials if auth_token is not None else None
    cookie_token = request.cookies.get("token") if "token" in request.cookies else None
    token = header_token if header_token is not None else cookie_token

    # Fallback to request.state.token (set by middleware, e.g. for x-api-key)
    if token is None and hasattr(request.state, "token") and request.state.token:
        token = request.state.token.credentials

    if token is None:
        raise HTTPException(status_code=401, detail="Not authenticated")
"""

patched = False
if old_token_block in source:
    source = source.replace(old_token_block, new_token_block, 1)
    patched = True
else:
    print("auth.py token selection block not found; skipping token patch")

old_except_block = """    except Exception as e:
        # Delete the token cookie
        if request.cookies.get("token"):
            response.delete_cookie("token")

        if request.cookies.get("oauth_id_token"):
            response.delete_cookie("oauth_id_token")

        # Delete OAuth session if present
        if request.cookies.get("oauth_session_id"):
            response.delete_cookie("oauth_session_id")

        raise e
"""

new_except_block = """    except Exception as e:
        # PAI_COOKIE_FALLBACK_PATCH: if Authorization is invalid but cookie is valid, keep session alive.
        if header_token and cookie_token and header_token != cookie_token:
            fallback = decode_token(cookie_token)
            if fallback is not None and "id" in fallback:
                if not fallback.get("jti") or await is_valid_token(request, fallback):
                    user = Users.get_user_by_id(fallback["id"])
                    if user is not None:
                        if background_tasks:
                            background_tasks.add_task(Users.update_last_active_by_id, user.id)
                        return user

        # Delete the token cookie
        if request.cookies.get("token"):
            response.delete_cookie("token")

        if request.cookies.get("oauth_id_token"):
            response.delete_cookie("oauth_id_token")

        # Delete OAuth session if present
        if request.cookies.get("oauth_session_id"):
            response.delete_cookie("oauth_session_id")

        raise e
"""

if old_except_block in source:
    source = source.replace(old_except_block, new_except_block, 1)
    patched = True
else:
    print("auth.py exception block not found; skipping exception patch")

if patched:
    path.write_text(source)
    print("patched auth.py with cookie fallback")
else:
    print("auth.py unchanged")
PY
