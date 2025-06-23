// cmd/localcloud/templates/chat/frontend/src/components/ModelSelector.jsx
import { Fragment } from 'react'
import { Listbox, Transition } from '@headlessui/react'
import clsx from 'clsx'

export default function ModelSelector({ models, selectedModel, onModelChange }) {
    const currentModel = models.find(m => m.name === selectedModel) || { name: selectedModel }

    return (
        <Listbox value={selectedModel} onChange={onModelChange}>
            <div className="relative">
                <Listbox.Button className="relative w-48 cursor-pointer rounded-lg bg-white dark:bg-gray-700 py-2 pl-3 pr-10 text-left shadow-sm ring-1 ring-inset ring-gray-300 dark:ring-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 sm:text-sm">
          <span className="block truncate text-gray-900 dark:text-white">
            {currentModel.name}
          </span>
                    <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
            <svg className="h-5 w-5 text-gray-400" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M10 3a1 1 0 01.707.293l3 3a1 1 0 01-1.414 1.414L10 5.414 7.707 7.707a1 1 0 01-1.414-1.414l3-3A1 1 0 0110 3zm-3.707 9.293a1 1 0 011.414 0L10 14.586l2.293-2.293a1 1 0 011.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clipRule="evenodd" />
            </svg>
          </span>
                </Listbox.Button>

                <Transition
                    as={Fragment}
                    leave="transition ease-in duration-100"
                    leaveFrom="opacity-100"
                    leaveTo="opacity-0"
                >
                    <Listbox.Options className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md bg-white dark:bg-gray-700 py-1 text-base shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none sm:text-sm">
                        {models.map((model) => (
                            <Listbox.Option
                                key={model.name}
                                className={({ active }) =>
                                    clsx(
                                        'relative cursor-pointer select-none py-2 pl-3 pr-9',
                                        active
                                            ? 'bg-blue-600 text-white'
                                            : 'text-gray-900 dark:text-white'
                                    )
                                }
                                value={model.name}
                            >
                                {({ selected, active }) => (
                                    <>
                    <span className={clsx('block truncate', selected && 'font-medium')}>
                      {model.name}
                    </span>
                                        {model.size && (
                                            <span
                                                className={clsx(
                                                    'block text-xs',
                                                    active ? 'text-blue-200' : 'text-gray-500 dark:text-gray-400'
                                                )}
                                            >
                        {formatSize(model.size)}
                      </span>
                                        )}
                                        {selected && (
                                            <span
                                                className={clsx(
                                                    'absolute inset-y-0 right-0 flex items-center pr-3',
                                                    active ? 'text-white' : 'text-blue-600'
                                                )}
                                            >
                        <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      </span>
                                        )}
                                    </>
                                )}
                            </Listbox.Option>
                        ))}
                    </Listbox.Options>
                </Transition>
            </div>
        </Listbox>
    )
}

function formatSize(bytes) {
    const gb = bytes / (1024 * 1024 * 1024)
    return `${gb.toFixed(1)} GB`
}