Please ensure the following rules are applied when creating a PR:

- Before creating your PR, execute the following command:
```bash
$> go vet
$> go lint
$> go fmt
$> go build
$> go test
```

- PR source branches shall have the following name: ***feature/<PR_NAME>***

- PR destination branch shall be develop

- Any PR not respecting those rules won’t be merge and will be closed after a few weeks if not updated.
