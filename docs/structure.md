# Library Structure for Chaos

- root folder (Library Name - camelCase)
    - default folder (default implementation created by the library)
        - all the modules, which includes everything else for the library should be included in here, be it a file or other folders for better segregation
    - all the interface modules, which includes interface folder (all the interfaces used necessary in the library - people can reimplement using the same format)
    - errors folder module, which includes all the errors used in the library and which is available for the users of the library
