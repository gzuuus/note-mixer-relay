package main

import (
	"html/template"
	"log"
	"net/http"
)

var templates *template.Template

func InitTemplates() {
	var err error
	templates, err = template.New("").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>{{.RelayName}}</title>
		<script src="https://unpkg.com/htmx.org@1.9.2"></script>
		<style>
			body {
				font-family: 'Inter', sans-serif;
				background-color: #1a1a1a;
				color: #e0e0e0;
				line-height: 1.6;
			}
			.container {
				max-width: 600px;
				margin: 0 auto;
				padding: 2rem;
				display: flex;
				flex-direction: column;
				align-items: center;
			}
			.card {
				background-color: #2a2a2a;
				border-radius: 0.5rem;
				padding: 1.5rem;
				margin-bottom: 1.5rem;
				box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
				width: 100%;
			}
			.input, .button {
				width: 90%;
				padding: 0.75rem;
				margin-bottom: 1rem;
				border-radius: 0.25rem;
				background-color: #3a3a3a;
				border: 1px solid #4a4a4a;
				color: #e0e0e0;
			}
			.button {
				background-color: #4a4aff;
				color: white;
				font-weight: bold;
				cursor: pointer;
				transition: background-color 0.3s;
			}
			.button:hover {
				background-color: #3a3aff;
			}
			.success { color: #4ade80; }
			.error { color: #f87171; }
			.warning { color: #fbbf24; }
		</style>
	</head>
	<body>
		<div class="container">
			<h1 class="text-4xl font-bold mb-8">{{.RelayName}}</h1>
			
			<div class="card">
				<h2 class="text-2xl font-semibold mb-4">Relay Information</h2>
				<p class="mb-2">{{.RelayDescription}}</p>
				<p class="mb-2"><strong>Allowed event kinds:</strong> {{.AllowedKinds}}</p>
				{{if .WhitelistEnabled}}
				<p class="mb-2">This relay uses a whitelist for pubkeys.</p>
				{{else}}
				<p class="mb-2">This relay is open to all pubkeys.</p>
				{{end}}
				<p class="mb-2"><strong>Connect to this relay using:</strong> <code class="bg-gray-800 px-2 py-1 rounded">ws://{{.Host}}/</code></p>
			</div>
			{{/*
			{{if not .WhitelistEnabled}}
			<div class="card">
				<h2 class="text-2xl font-semibold mb-4">Submit a Note</h2>
				<form hx-post="/submit-note" hx-target="#result">
					<textarea name="content" placeholder="Enter your note content" class="input" rows="4"></textarea>
					<button type="submit" class="button">Submit Note</button>
				</form>
				<div id="result" class="mt-4"></div>
			</div>
			{{end}}*/}}
		</div>
	</body>
	</html>
	`)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
}

func RenderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html")
	err := templates.ExecuteTemplate(w, templateName, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
