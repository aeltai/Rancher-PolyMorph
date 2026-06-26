package backup

import "fmt"

type RestorePlanOptions struct {
	Name          string
	BackupFile    string
	Encryption    string
	StorageSecret string
}

func FormatRestoreCR(opts RestorePlanOptions) string {
	name := opts.Name
	if name == "" {
		name = "rancher-restore"
	}
	backupFile := opts.BackupFile
	if backupFile == "" {
		backupFile = "sanitized-backup.tar.gz"
	}

	encLine := ""
	if opts.Encryption != "" {
		encLine = fmt.Sprintf("  encryptionConfigSecretName: %s\n", opts.Encryption)
	}

	storageBlock := ""
	if opts.StorageSecret != "" {
		storageBlock = fmt.Sprintf(`  storageLocation:
    s3:
      credentialSecretName: %s
      credentialSecretNamespace: cattle-resources-system
`, opts.StorageSecret)
	}

	return fmt.Sprintf(`# Restore manifest for Rancher migration (prune: false per Rancher docs)
# https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/backup-restore-and-disaster-recovery/migrate-rancher-to-new-cluster
#
# Prerequisites:
#   - rancher-backup operator installed on destination
#   - Rancher NOT running on destination
#   - backup tarball on operator PVC path (e.g. /var/lib/rancher-backups/)
apiVersion: resources.cattle.io/v1
kind: Restore
metadata:
  name: %s
spec:
  backupFilename: %s
  prune: false
%s%s`, name, backupFile, encLine, storageBlock)
}
