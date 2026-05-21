package tui

// screen_hints.go implements ScreenWithHints for each navigable screen model.
// Each Hints() returns a flat alternating [key, desc, …] slice consumed by
// renderStatusBar. Keeping all implementations here makes the full hint map
// visible at a glance and easy to audit for consistency.

func (m licensesModel) Hints() []string {
	return []string{
		"n", "nouvelle",
		"e", "re-émettre",
		"x", "révoquer",
		"d", "détail",
		"f", "filtrer",
		"q", "quitter",
	}
}

func (m issuersModel) Hints() []string {
	return []string{
		"n", "nouveau",
		"e", "éditer",
		"x", "retirer",
		"d", "détail",
		"q", "quitter",
	}
}

func (m recipientsModel) Hints() []string {
	return []string{
		"n", "nouveau",
		"e", "éditer",
		"x", "supprimer",
		"d", "détail",
		"q", "quitter",
	}
}

func (m identitiesModel) Hints() []string {
	return []string{
		"n", "nouvelle",
		"e", "éditer",
		"x", "supprimer",
		"d", "détail",
		"q", "quitter",
	}
}

func (m revocationModel) Hints() []string {
	return []string{
		"n", "ajouter entrée CRL",
		"x", "retirer",
		"d", "détail",
		"r", "rafraîchir",
		"q", "quitter",
	}
}

func (m serversModel) Hints() []string {
	return []string{
		"s", "start/stop",
		"A", "tout démarrer",
		"Z", "tout arrêter",
		"g", "regen token",
		"q", "quitter",
	}
}

func (m auditModel) Hints() []string {
	return []string{
		"d", "détail",
		"/", "chercher",
		"f", "filtrer",
		"r", "rafraîchir",
		"q", "quitter",
	}
}

func (m settingsModel) Hints() []string {
	return []string{
		"r", "rafraîchir",
		"P", "passphrase",
		"V", "vacuum",
		"B", "backup",
		"q", "quitter",
	}
}
