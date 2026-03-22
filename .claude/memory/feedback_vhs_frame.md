---
name: feedback_vhs_frame
description: When asked to check a VHS frame, use awk to extract from .log file
type: feedback
---

ユーザーが「frame N を確認して」と言ったら、以下のコマンドで取得する:

```bash
awk '/\[Frame N\]/,/\[Frame M\]/{if(/\[Frame M\]/)exit; print}' vis_e2e_tests/outputs/TAPE/TAPE.log
```

N = 指定フレーム、M = N+1。TAPE はテスト名。

**Why:** .log はプレーンテキスト。Read ツールで全体を読むとコンテキストを圧迫する。awk で該当フレームだけ取れば最小限。

**How to apply:** `vis_e2e_tests/outputs/` 以下の `.log` ファイルに対して awk を実行。Bash ツールで。
