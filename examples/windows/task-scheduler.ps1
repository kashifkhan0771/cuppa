# Post a cuppa question every weekday at 10:00 with Windows Task Scheduler.
# Run in PowerShell. The webhook goes into a user environment variable,
# so it is not written in the task or in any file you share.
#
#   1. Download cuppa.exe from the releases page into C:\Tools\cuppa\
#   2. Save your config as C:\Tools\cuppa\cuppa.yaml (see ..\cuppa.minimal.yaml)
#   3. Edit the webhook below and run this script
#
# Test it now:   Start-ScheduledTask -TaskName cuppa
# Remove it:     Unregister-ScheduledTask -TaskName cuppa -Confirm:$false

$webhook = "https://hooks.slack.com/services/..."
[Environment]::SetEnvironmentVariable("SLACK_WEBHOOK", $webhook, "User")

$action = New-ScheduledTaskAction `
    -Execute "C:\Tools\cuppa\cuppa.exe" `
    -Argument "post --config C:\Tools\cuppa\cuppa.yaml"
$trigger = New-ScheduledTaskTrigger -Weekly -DaysOfWeek Monday,Tuesday,Wednesday,Thursday,Friday -At 10:00
# run late if the computer was off at 10:00
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable

Register-ScheduledTask -TaskName "cuppa" -Action $action -Trigger $trigger -Settings $settings
