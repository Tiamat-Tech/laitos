# The setup script installs laitos supplements for windows, and a scheduled task that starts laitos automatically.

$ErrorActionPreference = 'Stop'

# Download and extract supplements (Firefox, SlimerJS) for laitos
$dataDrive = Read-Host -Prompt 'Under which drive will laitos supplements be installed? E.g. C:\'
$supplementsURL = 'https://github.com/HouzuoGuo/laitos-windows-supplements/archive/master.zip'
$supplementsSaveTo = $dataDrive + 'laitos-windows-supplements.zip'
$supplementsDest = $dataDrive + 'laitos-windows-supplements'
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12;
(New-Object Net.WebClient).DownloadFile($supplementsURL, $supplementsSaveTo)
$extractedSupplements = (New-Object -Com shell.application).namespace($supplementsSaveTo).Items()
(New-Object -Com shell.application).namespace($dataDrive).CopyHere($extractedSupplements, 16)
Remove-Item -ErrorAction Ignore -Recurse "$supplementsDest"
Rename-Item "${dataDrive}laitos-windows-supplements-master" $supplementsDest

# Run laitos automatically as system boots up via task scheduler
$laitosCmd = Read-Host -Prompt 'What is the absolute path to laitos.exe? E.g. %USERPROFILE%\laitos.exe'
$laitosArg = Read-Host -Prompt 'What parameters to use for launching laitos automatically? E.g. -disableconflicts -awsinteg -prominteg -gomaxprocs 2 -config config.json -daemons autounlock,dnsd,httpd,httpproxy,insecurehttpd,maintenance,passwdrpc,phonehome,plainsocket,serialport,simpleipsvcd,smtpd,snmpd,sockd,telegram'
$laitosWD = Read-Host -Prompt 'Which directory does laitos program data (JSON config, web pages, etc) reside?'
$laitosAction = New-ScheduledTaskAction -Execute $laitosCmd -Argument $laitosArg -WorkingDirectory $laitosWD
$laitosTrigger = New-ScheduledTaskTrigger -AtStartup
$laitosSettings = New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -RunOnlyIfNetworkAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -DontStopOnIdleEnd -RestartInterval (New-TimeSpan -Minutes 1) -RestartCount 100 -ExecutionTimeLimit (New-TimeSpan -Days 3650)
$laitosTask = New-ScheduledTask -Action $laitosAction -Trigger $laitosTrigger -Settings $laitosSettings
$laitosUser = Read-Host -Prompt 'What administrator user will laitos run as? E.g. Administrator'
$laitosPassword = Read-Host -AsSecureString -Prompt 'What is the administrator password?'
$laitosCred = New-Object System.Management.Automation.PSCredential -ArgumentList $laitosUser, $laitosPassword
$laitosTask | Register-ScheduledTask -Force -TaskName laitos -User $laitosUser -Password $laitosCred.GetNetworkCredential().Password

Read-Host -Prompt 'laitos is now ready to start automatically upon system boot, enter anything to terminate the setup script.'
