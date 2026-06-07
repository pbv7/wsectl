# Examples

## Identity And Users

```bash
wsectl me --json
wsectl users list --json
wsectl users schedule --start 01.05.2026 --end 31.05.2026 --json
```

## Projects

```bash
wsectl projects list --status active --extra text,options,users --json
wsectl projects get 123 --extra text,users --json
wsectl projects events --project 123 --period month --json
```

## Tasks

```bash
wsectl tasks all --status active --extra text,files --json --out /tmp/tasks.json
wsectl tasks list --project 123 --extra text,comments --json
wsectl tasks search --query "invoice" --json
wsectl tasks discussion 456 --json
```

## Files And Costs

```bash
wsectl comments list 456 --extra files --json
wsectl files list --project 123 --json
wsectl files download 789 --out ./file.bin
wsectl costs total --project 123 --start 01.05.2026 --end 31.05.2026 --json
```

## API Escape Hatch

```bash
wsectl api actions --json
wsectl api schema get_users_schedule --json
wsectl api call get_users_schedule --param datestart=01.05.2026 --param dateend=31.05.2026 --json
```

