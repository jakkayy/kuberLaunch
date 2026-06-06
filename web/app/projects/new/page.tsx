import CreateForm from './CreateForm'

export default function NewProjectPage() {
  return (
    <div className="max-w-lg">
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-zinc-900">New Project</h1>
        <p className="text-sm text-zinc-500 mt-1">
          Generate DevOps infrastructure in one click
        </p>
      </div>
      <CreateForm />
    </div>
  )
}
